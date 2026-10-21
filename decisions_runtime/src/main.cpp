#include "ggml-backend.h"
#include "ggml-alloc.h"
#include "ggml-cpu.h"
#include "gguf.h"
#include "llama.h"
#include "llama-ext.h"
#include "llama-model.h"
#include "mtmd.h"
#include "mtmd-helper.h"
extern "C" {
#include "sha256.h"
}

#include <httplib.h>
#include <nlohmann/json.hpp>
#include <stb_image.h>

#include <algorithm>
#include <array>
#include <atomic>
#include <chrono>
#include <cctype>
#include <cmath>
#include <cstdint>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <iterator>
#include <limits>
#include <memory>
#include <mutex>
#include <optional>
#include <set>
#include <sstream>
#include <stdexcept>
#include <string>
#include <string_view>
#include <thread>
#include <unordered_map>
#include <utility>
#include <vector>

#if defined(_WIN32)
#define NOMINMAX
#include <windows.h>
#else
#include <unistd.h>
#if defined(__APPLE__)
#include <mach-o/dyld.h>
#endif
#endif

namespace {
using json = nlohmann::json;
constexpr char kProtocol[] = "aurago.decisions.v1";
constexpr char kModelID[] = "jeff-2b-gguf";
constexpr char kRevision[] = "263ac27c59688a360d2db255c4f7e57f665bcca4";
constexpr char kModelSHA[] = "d4d9fbcf40345a1677dc92596c7c1e157fac5cba98e2a1b14ec92db94c67d46b";
constexpr char kProjectorSHA[] = "1601a51a07942318aacb7a7298847fe505f8beb592be37f538ab85577772bb68";
constexpr uint64_t kModelSize = 1274396448ULL;
constexpr uint64_t kProjectorSize = 671372608ULL;
constexpr size_t kContextTokens = 8192;
constexpr size_t kMaxRequestBytes = 40 * 1024 * 1024;
constexpr size_t kMaxCancelBytes = 4096;
constexpr size_t kMaxImageBytes = 12 * 1024 * 1024;
constexpr size_t kMaxTotalImageBytes = 24 * 1024 * 1024;
constexpr size_t kMaxImages = 8;
constexpr size_t kMaxImagePixels = 32 * 1024 * 1024;
constexpr size_t kMaxCandidates = 62;
constexpr size_t kHttpWorkers = 4;
constexpr size_t kHttpQueuedRequests = 4;
constexpr char kBackend[] = AURAGO_RUNTIME_BACKEND;
constexpr char kLlamaCommit[] = AURAGO_LLAMA_CPP_COMMIT;
constexpr char kLlamaPatchSHA[] = AURAGO_LLAMA_CPP_PATCH_SHA256;

struct Config {
    std::filesystem::path model_path;
    std::filesystem::path projector_path;
    std::string model_revision;
    std::string model_sha;
    std::string projector_revision;
    std::string projector_sha;
    std::string device_id;
    std::string bind = "127.0.0.1";
    int port = 0;
    std::filesystem::path token_file;
    bool allow_cpu = false;
    bool allow_cpu_offload = false;
    int gpu_layers = -1;
    bool discover = false;
    bool self_test = false;
};

struct RequestCancellation {
    std::atomic<bool> cancelled{false};
    const httplib::Request * request = nullptr;
};

struct PreflightProbe {
    size_t calls = 0;
    bool allow = false;
};

bool preflight_probe_callback(ggml_tensor *, ggml_backend_t, void * user_data) {
    auto * probe = static_cast<PreflightProbe *>(user_data);
    ++probe->calls;
    return probe->allow;
}

struct Placement {
    ggml_backend_dev_t selected = nullptr;
    size_t seen = 0;
    size_t cpu = 0;
    size_t preflight_seen = 0;
    bool all_on_selected = true;
    bool all_on_cpu = true;
    bool unknown = false;
    bool strict = false;
    bool preflight_rejected = false;
    RequestCancellation * cancellation = nullptr;
};

struct Offload {
    bool attested = false;
    bool model_layers_on_device = false;
    bool model_layers_on_cpu = false;
    bool projector_on_device = false;
    bool projector_compute_on_device = false;
    bool projector_weights_on_cpu = false;
    bool projector_compute_on_cpu = false;
    size_t projector_weight_bytes = 0;
    size_t projector_other_weight_bytes = 0;
    size_t projector_compute_bytes = 0;
    size_t projector_other_compute_bytes = 0;
    bool kv_on_device = false;
    bool model_cpu_compute = false;
    bool projector_cpu_compute = false;
    bool kv_cpu = false;
};

struct CandidateTokenError : std::exception {};
struct RequestCancelledError : std::exception {};
struct RuntimeBusyError : std::exception {};

void quiet_llama(enum ggml_log_level, const char *, void *) {}
void quiet_mtmd(enum ggml_log_level, const char *, void *) {}

bool cancellation_requested(RequestCancellation * cancellation) {
    if (!cancellation) return false;
    return cancellation->cancelled.load(std::memory_order_relaxed) ||
        (cancellation->request && cancellation->request->is_connection_closed());
}

bool abort_if_cancelled(void * user_data) {
    return cancellation_requested(static_cast<RequestCancellation *>(user_data));
}

bool ggml_metadata_only_op(enum ggml_op op) {
    switch (op) {
        case GGML_OP_NONE:
        case GGML_OP_RESHAPE:
        case GGML_OP_VIEW:
        case GGML_OP_PERMUTE:
        case GGML_OP_TRANSPOSE:
            return true;
        default:
            return false;
    }
}

bool preflight_node(ggml_tensor * tensor, ggml_backend_t backend, void * user_data) {
    auto * placement = static_cast<Placement *>(user_data);
    if (cancellation_requested(placement->cancellation)) {
        placement->preflight_rejected = true;
        return false;
    }
    // Host-resident input placeholders and graph shape aliases do not execute
    // model computation and are safe staging for a device backend.
    if (tensor && ggml_metadata_only_op(tensor->op)) return true;
    ++placement->preflight_seen;
    ++placement->seen;
    const auto dev = backend ? ggml_backend_get_device(backend) : nullptr;
    if (!dev) {
        placement->unknown = true;
        placement->preflight_rejected = true;
        placement->all_on_selected = false;
        placement->all_on_cpu = false;
        return false;
    }
    placement->all_on_selected = placement->all_on_selected && dev == placement->selected;
    const bool is_cpu = ggml_backend_dev_type(dev) == GGML_BACKEND_DEVICE_TYPE_CPU;
    placement->all_on_cpu = placement->all_on_cpu && is_cpu;
    if (is_cpu) ++placement->cpu;
    if (dev != placement->selected) {
        if (placement->strict) placement->preflight_rejected = true;
        return !placement->strict;
    }
    return true;
}

std::string lower(std::string s) {
    std::transform(s.begin(), s.end(), s.begin(), [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return s;
}

std::string hex(const unsigned char * data, size_t size) {
    static constexpr char digits[] = "0123456789abcdef";
    std::string out(size * 2, '0');
    for (size_t i = 0; i < size; ++i) {
        out[2 * i] = digits[data[i] >> 4];
        out[2 * i + 1] = digits[data[i] & 15];
    }
    return out;
}

std::string sha256_file(const std::filesystem::path & path, uint64_t * size_out = nullptr) {
    std::ifstream input(path, std::ios::binary);
    if (!input) throw std::runtime_error("artifact_unreadable");
    sha256_t state;
    sha256_init(&state);
    std::vector<unsigned char> block(64 * 1024);
    uint64_t total = 0;
    while (input) {
        input.read(reinterpret_cast<char *>(block.data()), static_cast<std::streamsize>(block.size()));
        const auto n = static_cast<size_t>(input.gcount());
        if (n) {
            sha256_update(&state, block.data(), n);
            total += n;
        }
    }
    if (!input.eof()) throw std::runtime_error("artifact_read_failed");
    unsigned char digest[SHA256_DIGEST_SIZE];
    sha256_final(&state, digest);
    if (size_out) *size_out = total;
    return hex(digest, sizeof(digest));
}

std::filesystem::path executable_path() {
#if defined(_WIN32)
    std::wstring path(32768, L'\0');
    const DWORD n = GetModuleFileNameW(nullptr, path.data(), static_cast<DWORD>(path.size()));
    if (n == 0 || n >= path.size()) throw std::runtime_error("executable_path_unavailable");
    path.resize(n);
    return std::filesystem::path(path);
#elif defined(__APPLE__)
    uint32_t size = 0;
    _NSGetExecutablePath(nullptr, &size);
    std::string path(size, '\0');
    if (_NSGetExecutablePath(path.data(), &size) != 0) throw std::runtime_error("executable_path_unavailable");
    path.resize(std::strlen(path.c_str()));
    return std::filesystem::canonical(path);
#else
    return std::filesystem::canonical("/proc/self/exe");
#endif
}

std::string binary_sha() {
    return sha256_file(executable_path());
}

std::string device_type_name(enum ggml_backend_dev_type type) {
    switch (type) {
        case GGML_BACKEND_DEVICE_TYPE_CPU: return "CPU";
        case GGML_BACKEND_DEVICE_TYPE_GPU: return "GPU";
        case GGML_BACKEND_DEVICE_TYPE_IGPU: return "IGPU";
        case GGML_BACKEND_DEVICE_TYPE_ACCEL: return "ACCEL";
        case GGML_BACKEND_DEVICE_TYPE_META: return "META";
        default: return "UNKNOWN";
    }
}

json device_json(ggml_backend_dev_t dev) {
    ggml_backend_dev_props props{};
    ggml_backend_dev_get_props(dev, &props);
    auto reg = ggml_backend_dev_backend_reg(dev);
    json integrated = nullptr;
    if (props.type == GGML_BACKEND_DEVICE_TYPE_IGPU) integrated = true;
    else if (props.type == GGML_BACKEND_DEVICE_TYPE_GPU) integrated = false;
    return {
        {"id", ggml_backend_dev_name(dev) ? ggml_backend_dev_name(dev) : ""},
        {"name", props.description ? props.description : (props.name ? props.name : "")},
        {"backend", reg && ggml_backend_reg_name(reg) ? ggml_backend_reg_name(reg) : "unknown"},
        {"type", device_type_name(props.type)},
        {"integrated", integrated},
        {"memory_total_bytes", props.memory_total},
        {"memory_free_bytes", props.memory_free},
    };
}

uint32_t model_layer_count(const std::filesystem::path & model_path) {
    gguf_init_params params{};
    params.no_alloc = true;
    gguf_context * metadata = gguf_init_from_file(model_path.string().c_str(), params);
    if (!metadata) throw std::runtime_error("model_metadata_unavailable");
    uint32_t result = 0;
    const int64_t arch_key = gguf_find_key(metadata, "general.architecture");
    if (arch_key >= 0 && gguf_get_kv_type(metadata, arch_key) == GGUF_TYPE_STRING) {
        const std::string key = std::string(gguf_get_val_str(metadata, arch_key)) + ".block_count";
        const int64_t layers_key = gguf_find_key(metadata, key.c_str());
        if (layers_key >= 0) {
            switch (gguf_get_kv_type(metadata, layers_key)) {
                case GGUF_TYPE_UINT32: result = gguf_get_val_u32(metadata, layers_key); break;
                case GGUF_TYPE_INT32: {
                    const int32_t value = gguf_get_val_i32(metadata, layers_key);
                    if (value > 0) result = static_cast<uint32_t>(value);
                    break;
                }
                case GGUF_TYPE_UINT64: {
                    const uint64_t value = gguf_get_val_u64(metadata, layers_key);
                    if (value <= 4096) result = static_cast<uint32_t>(value);
                    break;
                }
                default: break;
            }
        }
    }
    gguf_free(metadata);
    if (result == 0 || result > 4096) throw std::runtime_error("model_metadata_unavailable");
    return result;
}

json discover_json(const std::filesystem::path & model_path = {}) {
    // Discovery may read pinned model metadata; keep backend/model paths out of
    // stderr while preserving its machine-readable JSON on stdout.
    llama_log_set(quiet_llama, nullptr);
    mtmd_helper_log_set(quiet_mtmd, nullptr);
    llama_backend_init();
    json devices = json::array();
    const size_t count = ggml_backend_dev_count();
    for (size_t i = 0; i < count; ++i) devices.push_back(device_json(ggml_backend_dev_get(i)));
    json result = {
        {"protocol", kProtocol},
        {"build", {{"llama_cpp_commit", kLlamaCommit}, {"llama_cpp_patch_sha256", kLlamaPatchSHA},
            {"backend", kBackend}, {"binary_sha256", binary_sha()}}},
        {"devices", std::move(devices)},
    };
    if (!model_path.empty()) {
        result["model_layers"] = model_layer_count(model_path);
    }
    return result;
}

bool constant_time_equal(std::string_view a, std::string_view b) {
    size_t diff = a.size() ^ b.size();
    const size_t n = std::max(a.size(), b.size());
    for (size_t i = 0; i < n; ++i) {
        const unsigned char x = i < a.size() ? static_cast<unsigned char>(a[i]) : 0;
        const unsigned char y = i < b.size() ? static_cast<unsigned char>(b[i]) : 0;
        diff |= static_cast<size_t>(x ^ y);
    }
    return diff == 0;
}

bool bearer_authorized(const httplib::Request & req, std::string_view expected) {
    constexpr std::string_view prefix = "Bearer ";
    const auto header = req.get_header_value("Authorization");
    return header.size() >= prefix.size() && header.compare(0, prefix.size(), prefix) == 0 &&
        constant_time_equal(std::string_view(header).substr(prefix.size()), expected);
}

json parse_request_json(const std::string & input) {
    try {
        return json::parse(input, [](int depth, json::parse_event_t event, json &) {
            if ((event == json::parse_event_t::object_start || event == json::parse_event_t::array_start) && depth >= 32) {
                throw std::invalid_argument("invalid_request");
            }
            return true;
        });
    } catch (const json::exception &) {
        throw std::invalid_argument("invalid_request");
    }
}

struct ValidatedScoreRequest {
    std::string request_id;
    std::string prompt;
    int64_t budget = 0;
};

bool valid_request_id_value(const std::string & id) {
    return id.size() >= 16 && id.size() <= 128 && std::all_of(id.begin(), id.end(), [](unsigned char c) {
        return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_';
    });
}

ValidatedScoreRequest validate_score_request(const json & body) {
    if (!body.is_object() || !body.contains("request_id") || !body["request_id"].is_string() ||
        !body.contains("prompt") || !body["prompt"].is_string() ||
        !body.contains("max_input_tokens") || !body["max_input_tokens"].is_number_integer()) throw std::invalid_argument("invalid_request");
    ValidatedScoreRequest out{body["request_id"].get<std::string>(), body["prompt"].get<std::string>(), body["max_input_tokens"].get<int64_t>()};
    if (!valid_request_id_value(out.request_id) || out.prompt.empty() || out.budget < 1 || out.budget > static_cast<int64_t>(kContextTokens)) {
        throw std::invalid_argument("invalid_request");
    }
    return out;
}

std::string read_token(const std::filesystem::path & path) {
    std::ifstream input(path, std::ios::binary);
    if (!input) throw std::runtime_error("token_file_unreadable");
    std::string token((std::istreambuf_iterator<char>(input)), std::istreambuf_iterator<char>());
    while (!token.empty() && (token.back() == '\n' || token.back() == '\r' || token.back() == ' ' || token.back() == '\t')) token.pop_back();
    if (token.size() < 32 || token.size() > 4096 || token.find_first_of("\r\n") != std::string::npos) throw std::runtime_error("token_file_invalid");
    return token;
}

std::string base64_decode(std::string_view input) {
    static constexpr std::string_view alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    if (input.size() % 4 != 0 || input.size() > ((kMaxImageBytes + 2) / 3) * 4 + 4) throw std::runtime_error("invalid_request");
    std::string out;
    out.reserve(input.size() / 4 * 3);
    uint32_t val = 0;
    int bits = -8;
    bool padding = false;
    for (char ch : input) {
        if (ch == '=') { padding = true; continue; }
        if (padding) throw std::runtime_error("invalid_request");
        const auto pos = alphabet.find(ch);
        if (pos == std::string_view::npos) throw std::runtime_error("invalid_request");
        val = (val << 6) | static_cast<uint32_t>(pos);
        bits += 6;
        if (bits >= 0) {
            out.push_back(static_cast<char>((val >> bits) & 0xff));
            bits -= 8;
        }
        if (out.size() > kMaxImageBytes) throw std::runtime_error("invalid_request");
    }
    if (out.empty()) throw std::runtime_error("invalid_request");
    return out;
}

struct BitmapOwner {
    mtmd_bitmap * value = nullptr;
    BitmapOwner() = default;
    explicit BitmapOwner(mtmd_bitmap * p) : value(p) {}
    BitmapOwner(BitmapOwner && other) noexcept : value(std::exchange(other.value, nullptr)) {}
    BitmapOwner & operator=(BitmapOwner && other) noexcept {
        if (this != &other) { mtmd_bitmap_free(value); value = std::exchange(other.value, nullptr); }
        return *this;
    }
    ~BitmapOwner() { mtmd_bitmap_free(value); }
    BitmapOwner(const BitmapOwner &) = delete;
    BitmapOwner & operator=(const BitmapOwner &) = delete;
};

struct RequestImage {
    std::string mime;
    std::string bytes;
    BitmapOwner bitmap;
};

bool mime_magic_ok(std::string_view mime, std::string_view b) {
    const auto byte = [&](size_t i) { return static_cast<unsigned char>(b[i]); };
    if (mime == "image/jpeg") return b.size() >= 3 && byte(0) == 0xff && byte(1) == 0xd8 && byte(2) == 0xff;
    if (mime == "image/png") return b.size() >= 8 && byte(0) == 0x89 && b.substr(1, 3) == "PNG";
    if (mime == "image/gif") return b.size() >= 6 && (b.substr(0, 6) == "GIF87a" || b.substr(0, 6) == "GIF89a");
    if (mime == "image/bmp") return b.size() >= 2 && b.substr(0, 2) == "BM";
    return false;
}

struct Candidate {
    std::string code;
    std::string token_text;
    llama_token token = -1;
};

struct Runtime {
    Config config;
    std::string bearer;
    std::string binary_hash;
    ggml_backend_dev_t device = nullptr;
    json device_info;
    llama_model * model = nullptr;
    llama_context * context = nullptr;
    mtmd_context * mtmd = nullptr;
    Placement model_ops;
    Placement projector_ops;
    Offload offload;
    bool ready = false;
    bool backend_initialized = false;
    std::string marker;
    std::mutex inference_mutex;
    mutable std::mutex metadata_mutex;
    std::mutex requests_mutex;
    std::unordered_map<std::string, std::shared_ptr<RequestCancellation>> active_requests;
    std::unordered_map<std::string, std::chrono::steady_clock::time_point> pre_cancelled_requests;

    explicit Runtime(Config c) : config(std::move(c)) {}
    ~Runtime() {
        mtmd_free(mtmd);
        llama_free(context);
        llama_model_free(model);
        if (backend_initialized) llama_backend_free();
    }

    void validate_artifacts() {
        uint64_t size = 0;
        std::cerr << "AURAGO_DECISIONS_STARTUP phase=verifying_model\n";
        if (config.model_revision != kRevision || lower(config.model_sha) != kModelSHA ||
            sha256_file(config.model_path, &size) != kModelSHA || size != kModelSize) {
            throw std::runtime_error("model_pin_mismatch");
        }
        std::cerr << "AURAGO_DECISIONS_STARTUP phase=verifying_projector\n";
        if (config.projector_revision != kRevision || lower(config.projector_sha) != kProjectorSHA ||
            sha256_file(config.projector_path, &size) != kProjectorSHA || size != kProjectorSize) {
            throw std::runtime_error("projector_pin_mismatch");
        }
    }

    void initialize() {
        if (config.allow_cpu == !config.device_id.empty()) throw std::runtime_error("select_exactly_one_device_or_allow_cpu");
        if (config.allow_cpu_offload && (config.allow_cpu || config.device_id.empty())) throw std::runtime_error("cpu_offload_requires_selected_gpu");
        if (!config.allow_cpu_offload && config.gpu_layers != -1) throw std::runtime_error("gpu_layers_requires_cpu_offload_consent");
        if (config.model_path.empty() || config.projector_path.empty() || config.token_file.empty() || config.port < 1 || config.port > 65535) {
            throw std::runtime_error("runtime_arguments_incomplete");
        }
        if (config.allow_cpu && std::string(kBackend) != "cpu") throw std::runtime_error("cpu_consent_requires_cpu_build");
        if (!config.allow_cpu && std::string(kBackend) == "cpu") throw std::runtime_error("cpu_build_requires_explicit_cpu_consent");
        validate_artifacts();
        bearer = read_token(config.token_file);
        binary_hash = binary_sha();
        llama_log_set(quiet_llama, nullptr);
        mtmd_helper_log_set(quiet_mtmd, nullptr);
        llama_backend_init();
        backend_initialized = true;

        device = config.allow_cpu ? ggml_backend_dev_by_type(GGML_BACKEND_DEVICE_TYPE_CPU) : ggml_backend_dev_by_name(config.device_id.c_str());
        if (!device) throw std::runtime_error("selected_device_not_found");
        const auto type = ggml_backend_dev_type(device);
        if (config.allow_cpu && type != GGML_BACKEND_DEVICE_TYPE_CPU) throw std::runtime_error("selected_device_not_cpu");
        if (!config.allow_cpu && (type == GGML_BACKEND_DEVICE_TYPE_CPU || type == GGML_BACKEND_DEVICE_TYPE_META)) throw std::runtime_error("selected_device_not_accelerator");
        auto reg = ggml_backend_dev_backend_reg(device);
        const std::string registry = reg && ggml_backend_reg_name(reg) ? lower(ggml_backend_reg_name(reg)) : "";
        if (!config.allow_cpu && registry != kBackend) throw std::runtime_error("selected_device_backend_mismatch");
        device_info = device_json(device);
        model_ops.selected = device;
        projector_ops.selected = device;
        model_ops.strict = !config.allow_cpu && !config.allow_cpu_offload;
        projector_ops.strict = model_ops.strict;

        auto model_params = llama_model_default_params();
        ggml_backend_dev_t devices[] = {device, nullptr};
        model_params.devices = config.allow_cpu ? nullptr : devices;
        model_params.n_gpu_layers = config.allow_cpu ? 0 : config.gpu_layers;
        model_params.split_mode = LLAMA_SPLIT_MODE_NONE;
        std::array<llama_model_tensor_buft_override, 2> tensor_overrides{};
        if (model_ops.strict) {
            tensor_overrides[0] = {".*", ggml_backend_dev_buffer_type(device)};
            tensor_overrides[1] = {nullptr, nullptr};
            model_params.tensor_buft_overrides = tensor_overrides.data();
            model_params.no_host = true;
        }
        std::cerr << "AURAGO_DECISIONS_STARTUP phase=loading_model\n";
        model = llama_model_load_from_file(config.model_path.string().c_str(), model_params);
        if (!model) throw std::runtime_error("model_load_failed");
        if (config.allow_cpu_offload && (config.gpu_layers < 1 || config.gpu_layers >= static_cast<int>(model->layers.size()))) throw std::runtime_error("gpu_layer_count_invalid");

        auto ctx_params = llama_context_default_params();
        ctx_params.n_ctx = static_cast<uint32_t>(kContextTokens);
        ctx_params.n_batch = 1024;
        ctx_params.n_ubatch = 512;
        ctx_params.n_seq_max = 1;
        ctx_params.offload_kqv = true;
        ctx_params.op_offload = true;
        ctx_params.preflight_callback = preflight_node;
        ctx_params.preflight_callback_data = &model_ops;
        context = llama_init_from_model(model, ctx_params);
        if (!context) throw std::runtime_error("context_init_failed");

        auto mtmd_params = mtmd_context_params_default();
        mtmd_params.use_gpu = !config.allow_cpu;
        mtmd_params.device = device;
        mtmd_params.n_threads = 4;
        mtmd_params.warmup = false;
        mtmd_params.preflight_callback = preflight_node;
        mtmd_params.preflight_callback_user_data = &projector_ops;
        std::cerr << "AURAGO_DECISIONS_STARTUP phase=loading_projector\n";
        mtmd = mtmd_init_from_file(config.projector_path.string().c_str(), model, mtmd_params);
        if (!mtmd) throw std::runtime_error("projector_load_failed");
        marker = mtmd_get_marker(mtmd) ? mtmd_get_marker(mtmd) : "";
        if (marker.empty()) throw std::runtime_error("image_marker_unavailable");
        std::cerr << "AURAGO_DECISIONS_STARTUP phase=warming_up\n";
        warmup_and_attest();
        if (!offload.attested) throw std::runtime_error("device_attestation_incomplete");
        if (!config.allow_cpu_offload && !config.allow_cpu && (!offload.model_layers_on_device || !offload.projector_on_device || !offload.kv_on_device ||
            offload.model_cpu_compute || offload.projector_cpu_compute || offload.kv_cpu)) {
            if (offload.model_cpu_compute || offload.projector_cpu_compute) throw std::runtime_error("unauthorized_cpu_compute");
            throw std::runtime_error("offload_unavailable");
        }
        ready = true;
    }

    void warmup_and_attest() {
        std::vector<unsigned char> pixels(224 * 224 * 3, 127);
        BitmapOwner bitmap(mtmd_bitmap_init(224, 224, pixels.data()));
        if (!bitmap.value) throw std::runtime_error("attestation_warmup_failed");
        const mtmd_bitmap * bitmap_list[] = {bitmap.value};
        const std::string prompt = marker;
        mtmd_input_text text{prompt.c_str(), prompt.size(), true, true};
        std::unique_ptr<mtmd_input_chunks, decltype(&mtmd_input_chunks_free)> chunks(mtmd_input_chunks_init(), mtmd_input_chunks_free);
        if (!chunks || mtmd_tokenize(mtmd, chunks.get(), &text, bitmap_list, 1) != 0) throw std::runtime_error("attestation_tokenize_failed");
        const auto tokens = mtmd_helper_get_n_tokens(chunks.get());
        if (tokens == 0 || tokens >= kContextTokens) throw std::runtime_error("attestation_context_invalid");
        llama_memory_clear(llama_get_memory(context), true);
        llama_pos new_past = 0;
        if (mtmd_helper_eval_chunks(mtmd, context, chunks.get(), 0, 0, 512, true, &new_past) != 0) {
            if (model_ops.preflight_rejected || projector_ops.preflight_rejected) {
                if (model_ops.cpu > 0 || projector_ops.cpu > 0) throw std::runtime_error("unauthorized_cpu_compute");
                throw std::runtime_error("offload_unavailable");
            }
            throw std::runtime_error("attestation_inference_failed");
        }
        update_attestation();
        llama_memory_clear(llama_get_memory(context), true);
    }

    void update_attestation() {
        std::lock_guard<std::mutex> guard(metadata_mutex);
        const bool selected_accelerator = device && ggml_backend_dev_type(device) != GGML_BACKEND_DEVICE_TYPE_CPU;
        bool layers_on_device = selected_accelerator && model && !model->layers.empty() && model->dev_output() == device;
        bool model_weights_on_cpu = model && !model->layers.empty();
        bool model_layers_on_cpu = model && !model->layers.empty() &&
            model->dev_output() && ggml_backend_dev_type(model->dev_output()) == GGML_BACKEND_DEVICE_TYPE_CPU;
        for (size_t i = 0; model && i < model->layers.size(); ++i) {
            const auto layer_device = model->dev_layer(static_cast<int>(i));
            layers_on_device = layers_on_device && layer_device == device;
            model_layers_on_cpu = model_layers_on_cpu && layer_device && ggml_backend_dev_type(layer_device) == GGML_BACKEND_DEVICE_TYPE_CPU;
        }
        size_t model_bytes = 0;
        for (const auto & [buft, bytes] : model->memory_breakdown()) {
            model_bytes += bytes;
            if (bytes) {
                const auto buffer_device = buft ? ggml_backend_buft_get_device(buft) : nullptr;
                if (!buffer_device || buffer_device != device) layers_on_device = false;
                if (!buffer_device || ggml_backend_dev_type(buffer_device) != GGML_BACKEND_DEVICE_TYPE_CPU) model_weights_on_cpu = false;
            }
        }
        layers_on_device = layers_on_device && model_bytes > 0;
        model_layers_on_cpu = model_layers_on_cpu && model_weights_on_cpu && model_bytes > 0;

        bool kv_seen = false;
        bool kv_on_device = selected_accelerator;
        bool kv_cpu = false;
        const auto memory = llama_get_memory_breakdown(context);
        for (const auto & [buft, bytes] : memory) {
            if (bytes.context == 0) continue;
            kv_seen = true;
            const auto dev = buft ? ggml_backend_buft_get_device(buft) : nullptr;
            if (!dev) kv_on_device = false;
            else if (dev != device) kv_on_device = false;
            if (dev && ggml_backend_dev_type(dev) == GGML_BACKEND_DEVICE_TYPE_CPU) kv_cpu = true;
        }
        const bool model_observed = model_ops.preflight_seen > 0 && !model_ops.unknown;
        const bool projector_observed = projector_ops.preflight_seen > 0 && !projector_ops.unknown;
        const bool kv_observed = kv_seen;
        const auto projector_memory = mtmd_get_context_device_memory_usage(mtmd, device);
        offload.projector_weight_bytes = projector_memory.selected_weight_bytes;
        offload.projector_other_weight_bytes = projector_memory.other_weight_bytes;
        offload.projector_compute_bytes = projector_memory.selected_compute_bytes;
        offload.projector_other_compute_bytes = projector_memory.other_compute_bytes;
        const bool projector_gpu_weights = selected_accelerator && projector_memory.selected_weight_bytes > 0 &&
            projector_memory.other_weight_bytes == 0 && projector_memory.non_cpu_other_weight_bytes == 0;
        // clip_get_compute_mem_usage reports allocated scratch-buffer capacity,
        // not where operators execute. The scheduler preflight records every
        // real graph operator's selected backend before compute and is the
        // source of truth for execution placement.
        const bool projector_gpu_compute = selected_accelerator && projector_observed && projector_ops.all_on_selected;
        // A selected CPU backend also appears in the selected-device counters.
        // For GPU/hybrid, a fully CPU projector must instead have no selected
        // bytes; for CPU-only, selected and CPU counters must agree exactly.
        const bool cpu_device_selected = !selected_accelerator;
        const bool projector_cpu_weights = projector_memory.cpu_weight_bytes > 0 && projector_memory.non_cpu_other_weight_bytes == 0 &&
            (cpu_device_selected ? projector_memory.selected_weight_bytes == projector_memory.cpu_weight_bytes : projector_memory.selected_weight_bytes == 0);
        const bool projector_cpu_compute = projector_observed && projector_ops.cpu > 0 && projector_ops.all_on_cpu;
        offload.model_layers_on_device = layers_on_device && model_ops.all_on_selected;
        offload.model_layers_on_cpu = model_layers_on_cpu && model_ops.all_on_cpu;
        offload.projector_on_device = projector_observed && projector_ops.all_on_selected && projector_gpu_weights;
        offload.projector_compute_on_device = projector_observed && projector_gpu_compute;
        offload.projector_weights_on_cpu = projector_observed && projector_cpu_weights;
        offload.projector_compute_on_cpu = projector_observed && projector_cpu_compute;
        offload.kv_on_device = kv_observed && kv_on_device;
        offload.kv_cpu = kv_cpu;
        offload.model_cpu_compute = model_ops.cpu > 0;
        offload.projector_cpu_compute = projector_ops.cpu > 0;
        const bool projector_placement_attested =
            (offload.projector_on_device && offload.projector_compute_on_device && !offload.projector_cpu_compute &&
                projector_memory.cpu_weight_bytes == 0) ||
            (offload.projector_weights_on_cpu && offload.projector_compute_on_cpu && offload.projector_cpu_compute &&
                projector_memory.non_cpu_other_weight_bytes == 0);
        offload.attested = model_observed && projector_observed && kv_observed && projector_placement_attested;
        if (config.allow_cpu) {
            offload.model_cpu_compute = model_observed && model_ops.cpu > 0 && model_ops.all_on_cpu;
            offload.projector_cpu_compute = offload.projector_weights_on_cpu && offload.projector_compute_on_cpu;
            offload.kv_cpu = kv_observed && kv_cpu;
            offload.attested = model_observed && projector_observed && kv_observed && offload.model_layers_on_cpu &&
                offload.model_cpu_compute && offload.projector_weights_on_cpu && offload.projector_compute_on_cpu &&
                offload.projector_cpu_compute && offload.kv_cpu;
        }
    }

    json model_pin() const { return {{"id", kModelID}, {"revision", kRevision}, {"sha256", kModelSHA}}; }
    json projector_pin() const { return {{"revision", kRevision}, {"sha256", kProjectorSHA}}; }
    json build_info() const { return {{"llama_cpp_commit", kLlamaCommit}, {"llama_cpp_patch_sha256", kLlamaPatchSHA},
        {"binary_sha256", binary_hash}, {"backend", kBackend}}; }
    json hardware_info() const {
        return {{"selected_device", device_info}, {"offload", {
            {"attested", offload.attested}, {"all_model_layers_on_device", offload.model_layers_on_device},
            {"all_model_layers_on_cpu", offload.model_layers_on_cpu},
            {"projector_on_device", offload.projector_on_device}, {"projector_compute_on_device", offload.projector_compute_on_device},
            {"projector_weights_on_cpu", offload.projector_weights_on_cpu}, {"projector_compute_on_cpu", offload.projector_compute_on_cpu},
            {"projector_weight_bytes", offload.projector_weight_bytes}, {"projector_other_weight_bytes", offload.projector_other_weight_bytes},
            {"projector_compute_bytes", offload.projector_compute_bytes}, {"projector_other_compute_bytes", offload.projector_other_compute_bytes},
            {"kv_on_device", offload.kv_on_device},
            {"model_cpu_compute", offload.model_cpu_compute}, {"projector_cpu_compute", offload.projector_cpu_compute}, {"kv_cpu", offload.kv_cpu},
        }}};
    }
    json limits() const {
        return {{"context_tokens", kContextTokens}, {"max_request_bytes", kMaxRequestBytes}, {"max_image_bytes", kMaxImageBytes},
            {"max_images", kMaxImages}, {"max_total_image_bytes", kMaxTotalImageBytes}};
    }
    json metadata() const {
        std::lock_guard<std::mutex> guard(metadata_mutex);
        return {{"protocol", kProtocol}, {"ready", ready}, {"model", model_pin()}, {"projector", projector_pin()},
            {"model_layers", model ? llama_model_n_layer(model) : 0},
            {"build", build_info()}, {"hardware", hardware_info()}, {"limits", limits()}, {"image_marker", marker},
            {"placement", {{"hybrid", config.allow_cpu_offload}, {"requested_gpu_layers", config.allow_cpu_offload ? config.gpu_layers : 0},
                {"model_layers_on_device", offload.model_layers_on_device}, {"model_cpu_compute", offload.model_cpu_compute},
                {"model_layers_on_cpu", offload.model_layers_on_cpu},
                {"projector_on_device", offload.projector_on_device}, {"projector_compute_on_device", offload.projector_compute_on_device},
                {"projector_weights_on_cpu", offload.projector_weights_on_cpu}, {"projector_compute_on_cpu", offload.projector_compute_on_cpu},
                {"projector_cpu_compute", offload.projector_cpu_compute},
                {"kv_on_device", offload.kv_on_device}, {"kv_cpu", offload.kv_cpu},
                {"total_model_layers", model ? llama_model_n_layer(model) : 0}}}};
    }

    bool authorized(const httplib::Request & req) const {
        return bearer_authorized(req, bearer);
    }

    static void error(httplib::Response & res, int status, const char * code, const std::string * request_id = nullptr) {
        res.status = status;
        json body = {{"error", {{"code", code}}}};
        if (request_id && !request_id->empty()) body["request_id"] = *request_id;
        res.set_content(body.dump(), "application/json");
    }

    static bool valid_request_id(const std::string & id) {
        return valid_request_id_value(id);
    }

    void cancel(const json & body, httplib::Response & res) {
        if (body.is_discarded() || !body.is_object() || !body.contains("request_id") || !body["request_id"].is_string()) {
            throw std::invalid_argument("invalid_request");
        }
        const std::string request_id = body["request_id"].get<std::string>();
        if (!valid_request_id(request_id)) throw std::invalid_argument("invalid_request");
        bool cancelled = false;
        {
            std::lock_guard<std::mutex> guard(requests_mutex);
            const auto now = std::chrono::steady_clock::now();
            for (auto it = pre_cancelled_requests.begin(); it != pre_cancelled_requests.end();) {
                if (it->second <= now) it = pre_cancelled_requests.erase(it);
                else ++it;
            }
            const auto it = active_requests.find(request_id);
            if (it != active_requests.end()) {
                it->second->cancelled.store(true, std::memory_order_relaxed);
                cancelled = true;
            } else {
                constexpr size_t max_pre_cancellations = 256;
                if (pre_cancelled_requests.size() >= max_pre_cancellations) {
                    auto oldest = std::min_element(pre_cancelled_requests.begin(), pre_cancelled_requests.end(),
                        [](const auto & a, const auto & b) { return a.second < b.second; });
                    if (oldest != pre_cancelled_requests.end()) pre_cancelled_requests.erase(oldest);
                }
                pre_cancelled_requests[request_id] = now + std::chrono::seconds(30);
                cancelled = true;
            }
        }
        res.status = 200;
        res.set_content(json{{"request_id", request_id}, {"cancelled", cancelled},
            {"error", nullptr}}.dump(), "application/json");
    }

    void attach_routes(httplib::Server & server) {
        server.set_payload_max_length(kMaxRequestBytes);
        server.set_read_timeout(15, 0);
        server.set_write_timeout(15, 0);
        server.set_pre_routing_handler([this](const httplib::Request & req, httplib::Response & res) {
            if (!authorized(req)) {
                error(res, 401, "unauthorized");
                res.set_header("Connection", "close");
                return httplib::Server::HandlerResponse::Handled;
            }
            const size_t limit = req.path == "/cancel" ? kMaxCancelBytes : kMaxRequestBytes;
            if (req.method == "POST" && req.get_header_value_u64("Content-Length") > limit) {
                error(res, 413, "invalid_request");
                res.set_header("Connection", "close");
                return httplib::Server::HandlerResponse::Handled;
            }
            return httplib::Server::HandlerResponse::Unhandled;
        });
        server.Get("/healthz", [this](const httplib::Request &, httplib::Response & res) {
            res.set_content(metadata().dump(), "application/json");
        });
        server.Get("/capabilities", [this](const httplib::Request &, httplib::Response & res) {
            res.set_content(metadata().dump(), "application/json");
        });
        server.Post("/score", [this](const httplib::Request & req, httplib::Response & res) {
            std::string request_id;
            try {
                const auto body = parse_request_json(req.body);
                if (body.is_object() && body.contains("request_id") && body["request_id"].is_string()) {
                    const auto candidate = body["request_id"].get<std::string>();
                    if (valid_request_id(candidate)) request_id = candidate;
                }
                score(req, res, body);
            }
            catch (const std::invalid_argument &) { error(res, 400, "invalid_request", &request_id); }
            catch (const std::length_error &) { error(res, 413, "input_token_budget_exceeded", &request_id); }
            catch (const CandidateTokenError &) { error(res, 422, "candidate_not_single_token", &request_id); }
            catch (const RequestCancelledError &) { error(res, 409, "request_cancelled", &request_id); }
            catch (const RuntimeBusyError &) { error(res, 503, "runtime_busy", &request_id); }
            catch (const std::runtime_error & e) {
                const std::string code = e.what();
                if (code == "runtime_not_ready") error(res, 503, "runtime_not_ready", &request_id);
                else if (code == "device_attestation_changed") error(res, 503, "device_attestation_changed", &request_id);
                else error(res, 500, "inference_failed", &request_id);
            }
            catch (...) { error(res, 500, "inference_failed", &request_id); }
        });
        server.Post("/cancel", [this](const httplib::Request &, httplib::Response & res, const httplib::ContentReader & reader) {
            std::string body;
            bool too_large = false;
            if (!reader([&](const char * data, size_t size) {
                if (size > kMaxCancelBytes - body.size()) { too_large = true; return false; }
                body.append(data, size);
                return true;
            })) {
                error(res, too_large ? 413 : 400, "invalid_request");
                res.set_header("Connection", "close");
                return;
            }
            try { cancel(parse_request_json(body), res); }
            catch (const std::invalid_argument &) { error(res, 400, "invalid_request"); }
        });
    }

    static std::vector<llama_token> tokenize(const llama_vocab * vocab, std::string_view text, bool add_special = false) {
        if (text.size() > static_cast<size_t>(std::numeric_limits<int32_t>::max())) throw std::invalid_argument("invalid_request");
        std::vector<llama_token> tokens(std::max<size_t>(32, text.size() + 8));
        int32_t n = llama_tokenize(vocab, text.data(), static_cast<int32_t>(text.size()), tokens.data(), static_cast<int32_t>(tokens.size()), add_special, true);
        if (n < 0) {
            tokens.resize(static_cast<size_t>(-n));
            n = llama_tokenize(vocab, text.data(), static_cast<int32_t>(text.size()), tokens.data(), static_cast<int32_t>(tokens.size()), add_special, true);
        }
        if (n < 0) throw std::invalid_argument("invalid_request");
        tokens.resize(static_cast<size_t>(n));
        return tokens;
    }

    std::vector<RequestImage> parse_images(const json & body, std::vector<const mtmd_bitmap *> & bitmaps) const {
        std::vector<RequestImage> images;
        size_t total = 0;
        if (!body.contains("images")) return images;
        if (!body["images"].is_array() || body["images"].size() > kMaxImages) throw std::invalid_argument("invalid_request");
        images.reserve(body["images"].size());
        for (const auto & item : body["images"]) {
            if (!item.is_object() || !item.contains("mime_type") || !item["mime_type"].is_string() ||
                !item.contains("data_base64") || !item["data_base64"].is_string()) throw std::invalid_argument("invalid_request");
            RequestImage image;
            image.mime = item["mime_type"].get<std::string>();
            image.bytes = base64_decode(item["data_base64"].get<std::string>());
            if (image.bytes.size() > kMaxImageBytes || image.bytes.size() > kMaxTotalImageBytes - total || !mime_magic_ok(image.mime, image.bytes)) {
                throw std::invalid_argument("invalid_request");
            }
            total += image.bytes.size();
            int w = 0, h = 0, channels = 0;
            if (!stbi_info_from_memory(reinterpret_cast<const stbi_uc *>(image.bytes.data()), static_cast<int>(image.bytes.size()), &w, &h, &channels) ||
                w <= 0 || h <= 0 || static_cast<uint64_t>(w) * static_cast<uint64_t>(h) > kMaxImagePixels) throw std::invalid_argument("invalid_request");
            auto wrapper = mtmd_helper_bitmap_init_from_buf(mtmd, reinterpret_cast<const unsigned char *>(image.bytes.data()), image.bytes.size(), false, mtmd_helper_init_opt_default());
            if (!wrapper.bitmap || wrapper.video_ctx) {
                if (wrapper.bitmap) mtmd_bitmap_free(wrapper.bitmap);
                throw std::invalid_argument("invalid_request");
            }
            image.bitmap = BitmapOwner(wrapper.bitmap);
            bitmaps.push_back(image.bitmap.value);
            images.push_back(std::move(image));
        }
        return images;
    }

    static size_t count_marker(std::string_view text, std::string_view marker) {
        size_t count = 0, pos = 0;
        while ((pos = text.find(marker, pos)) != std::string_view::npos) { ++count; pos += marker.size(); }
        return count;
    }

    std::vector<Candidate> parse_candidates(const json & body, std::string_view prompt) const {
        if (!body.contains("candidates") || !body["candidates"].is_array() || body["candidates"].size() < 2 || body["candidates"].size() > kMaxCandidates) throw std::invalid_argument("invalid_request");
        const auto marker_pos = prompt.rfind(marker);
        const std::string_view tail = marker_pos == std::string_view::npos ? prompt : prompt.substr(marker_pos + marker.size());
        auto prefix_tokens = tokenize(llama_model_get_vocab(model), tail, false);
        std::set<std::string> codes;
        std::set<llama_token> token_ids;
        std::vector<Candidate> candidates;
        candidates.reserve(body["candidates"].size());
        for (const auto & item : body["candidates"]) {
            if (!item.is_object() || !item.contains("code") || !item["code"].is_string() || !item.contains("token_text") || !item["token_text"].is_string()) throw std::invalid_argument("invalid_request");
            Candidate c{item["code"].get<std::string>(), item["token_text"].get<std::string>(), -1};
            if (c.code.empty() || c.code.size() > 128 || c.token_text.size() != 1 ||
                std::string_view("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789").find(c.token_text[0]) == std::string_view::npos ||
                !codes.insert(c.code).second) throw std::invalid_argument("invalid_request");
            auto with = tokenize(llama_model_get_vocab(model), std::string(tail) + c.token_text, false);
            if (with.size() != prefix_tokens.size() + 1 || !std::equal(prefix_tokens.begin(), prefix_tokens.end(), with.begin())) {
                throw CandidateTokenError();
            }
            c.token = with.back();
            if (c.token < 0 || static_cast<size_t>(c.token) >= llama_vocab_n_tokens(llama_model_get_vocab(model)) || !token_ids.insert(c.token).second) throw CandidateTokenError();
            candidates.push_back(std::move(c));
        }
        return candidates;
    }

    void score(const httplib::Request & req, httplib::Response & res, const json & body) {
        const auto validated = validate_score_request(body);
        const std::string & request_id = validated.request_id;
        const std::string & prompt = validated.prompt;
        const int64_t budget = validated.budget;

        auto cancellation = std::make_shared<RequestCancellation>();
        cancellation->request = &req;
        {
            std::lock_guard<std::mutex> guard(requests_mutex);
            const auto now = std::chrono::steady_clock::now();
            for (auto it = pre_cancelled_requests.begin(); it != pre_cancelled_requests.end();) {
                if (it->second <= now) it = pre_cancelled_requests.erase(it);
                else ++it;
            }
            const auto cancelled_it = pre_cancelled_requests.find(request_id);
            if (cancelled_it != pre_cancelled_requests.end()) {
                cancellation->cancelled.store(true, std::memory_order_relaxed);
                pre_cancelled_requests.erase(cancelled_it);
            }
            if (!active_requests.emplace(request_id, cancellation).second) throw std::invalid_argument("invalid_request");
        }
        struct RequestRegistration {
            Runtime * runtime;
            std::string id;
            ~RequestRegistration() {
                std::lock_guard<std::mutex> guard(runtime->requests_mutex);
                runtime->active_requests.erase(id);
            }
        } registration{this, request_id};

        std::unique_lock<std::mutex> lock(inference_mutex, std::try_to_lock);
        if (!lock.owns_lock()) {
            if (cancellation_requested(cancellation.get())) throw RequestCancelledError();
            throw RuntimeBusyError();
        }
        if (cancellation_requested(cancellation.get())) throw RequestCancelledError();
        { std::lock_guard<std::mutex> guard(metadata_mutex); if (!ready) throw std::runtime_error("runtime_not_ready"); }
        model_ops.cancellation = cancellation.get();
        projector_ops.cancellation = cancellation.get();
        model_ops.preflight_rejected = false;
        projector_ops.preflight_rejected = false;
        llama_set_abort_callback(context, abort_if_cancelled, cancellation.get());
        struct CallbackReset {
            Runtime * runtime;
            ~CallbackReset() {
                llama_set_abort_callback(runtime->context, nullptr, nullptr);
                runtime->model_ops.cancellation = nullptr;
                runtime->projector_ops.cancellation = nullptr;
            }
        } callback_reset{this};

        std::vector<const mtmd_bitmap *> bitmaps;
        auto images = parse_images(body, bitmaps);
        if (count_marker(prompt, marker) != images.size()) throw std::invalid_argument("invalid_request");
        auto candidates = parse_candidates(body, prompt);
        mtmd_input_text text{prompt.c_str(), prompt.size(), true, true};
        std::unique_ptr<mtmd_input_chunks, decltype(&mtmd_input_chunks_free)> chunks(mtmd_input_chunks_init(), mtmd_input_chunks_free);
        if (!chunks || mtmd_tokenize(mtmd, chunks.get(), &text, bitmaps.data(), bitmaps.size()) != 0) throw std::invalid_argument("invalid_request");
        size_t prompt_tokens = 0, image_tokens = 0;
        for (size_t i = 0; i < mtmd_input_chunks_size(chunks.get()); ++i) {
            const auto * chunk = mtmd_input_chunks_get(chunks.get(), i);
            if (mtmd_input_chunk_get_type(chunk) == MTMD_INPUT_CHUNK_TYPE_IMAGE) image_tokens += mtmd_input_chunk_get_n_tokens(chunk);
            else if (mtmd_input_chunk_get_type(chunk) == MTMD_INPUT_CHUNK_TYPE_TEXT) prompt_tokens += mtmd_input_chunk_get_n_tokens(chunk);
            else throw std::invalid_argument("invalid_request");
        }
        const size_t total_tokens = prompt_tokens + image_tokens;
        if (total_tokens > static_cast<size_t>(budget) || total_tokens > kContextTokens) throw std::length_error("input_token_budget_exceeded");
        llama_memory_clear(llama_get_memory(context), true);
        llama_pos new_past = 0;
        if (mtmd_helper_eval_chunks(mtmd, context, chunks.get(), 0, 0, 512, true, &new_past) != 0) {
            if (cancellation_requested(cancellation.get())) throw RequestCancelledError();
            if (model_ops.preflight_rejected || projector_ops.preflight_rejected) throw std::runtime_error("strict_device_preflight_rejected");
            throw std::runtime_error("inference_failed");
        }
        if (cancellation_requested(cancellation.get())) throw RequestCancelledError();
        auto * logits = llama_get_logits_ith(context, -1);
        if (!logits) throw std::runtime_error("inference_failed");
        json scores = json::array();
        for (const auto & candidate : candidates) {
            const float value = logits[candidate.token];
            if (!std::isfinite(value)) throw std::runtime_error("inference_failed");
            scores.push_back({{"code", candidate.code}, {"token_id", candidate.token}, {"logit", value}});
        }
        update_attestation();
        if (!offload.attested || (!config.allow_cpu_offload && !config.allow_cpu && (!offload.model_layers_on_device || !offload.projector_on_device || !offload.kv_on_device || offload.model_cpu_compute || offload.projector_cpu_compute || offload.kv_cpu))) {
            { std::lock_guard<std::mutex> guard(metadata_mutex); ready = false; }
            throw std::runtime_error("device_attestation_changed");
        }
        json response = metadata();
        response["request_id"] = request_id;
        response["usage"] = {{"prompt_tokens", prompt_tokens}, {"image_tokens", image_tokens}, {"context_tokens", total_tokens}};
        response["scores"] = std::move(scores);
        res.set_content(response.dump(), "application/json");
    }

    void serve() {
        httplib::Server server;
        server.new_task_queue = [] {
            return new httplib::ThreadPool(kHttpWorkers, kHttpWorkers, kHttpQueuedRequests);
        };
        attach_routes(server);
        if (!server.bind_to_port(config.bind, config.port)) throw std::runtime_error("http_bind_failed");
        std::cout << "{" << "\"event\":\"listening\",\"bind\":\"" << config.bind << "\",\"port\":" << config.port << "}" << std::endl;
        if (!server.listen_after_bind()) throw std::runtime_error("http_listen_failed");
    }
};

Config parse_args(int argc, char ** argv) {
    Config c;
    auto value = [&](int & i, const char * flag) -> std::string {
        if (i + 1 >= argc) throw std::runtime_error(std::string("missing_value_for_") + flag);
        return argv[++i];
    };
    for (int i = 1; i < argc; ++i) {
        const std::string arg = argv[i];
        if (arg == "--model") c.model_path = value(i, "--model");
        else if (arg == "--model-revision") c.model_revision = value(i, "--model-revision");
        else if (arg == "--model-sha256") c.model_sha = value(i, "--model-sha256");
        else if (arg == "--mmproj") c.projector_path = value(i, "--mmproj");
        else if (arg == "--mmproj-revision") c.projector_revision = value(i, "--mmproj-revision");
        else if (arg == "--mmproj-sha256") c.projector_sha = value(i, "--mmproj-sha256");
        else if (arg == "--device") c.device_id = value(i, "--device");
        else if (arg == "--bind") c.bind = value(i, "--bind");
        else if (arg == "--token-file") c.token_file = value(i, "--token-file");
        else if (arg == "--port") c.port = std::stoi(value(i, "--port"));
        else if (arg == "--allow-cpu") c.allow_cpu = true;
        else if (arg == "--allow-cpu-offload") c.allow_cpu_offload = true;
        else if (arg == "--gpu-layers") c.gpu_layers = std::stoi(value(i, "--gpu-layers"));
        else if (arg == "--discover") c.discover = true;
        else if (arg == "--self-test-protocol") c.self_test = true;
        else throw std::runtime_error("unknown_runtime_argument");
    }
    if (c.discover && (argc == 2)) return c;
    if (c.self_test && (argc == 2)) return c;
    return c;
}

int self_test_protocol() {
    std::string configured = "0123456789abcdef0123456789abcdef";
    Runtime runtime(Config{});
    runtime.bearer = configured;
    httplib::Server server;
    server.new_task_queue = [] { return new httplib::ThreadPool(kHttpWorkers, kHttpWorkers, kHttpQueuedRequests); };
    runtime.attach_routes(server);
    const int port = server.bind_to_any_port("127.0.0.1");
    if (port <= 0) throw std::runtime_error("protocol_self_test_failed");
    std::thread worker([&server] { server.listen_after_bind(); });
    for (int i = 0; i < 100 && !server.is_running(); ++i) std::this_thread::sleep_for(std::chrono::milliseconds(10));
    bool passed = false;
    try {
        httplib::Client client("127.0.0.1", port);
        client.set_connection_timeout(1, 0);
        httplib::Headers good{{"Authorization", "Bearer " + configured}};
        auto unauthorized = client.Get("/healthz");
        if (!unauthorized || unauthorized->status != 401 || json::parse(unauthorized->body)["error"]["code"] != "unauthorized") throw std::runtime_error("protocol_self_test_failed");
        auto health = client.Get("/healthz", good);
        if (!health || health->status != 200 || json::parse(health->body)["protocol"] != kProtocol) throw std::runtime_error("protocol_self_test_failed");
        auto denied = client.Post("/score", R"({"request_id":"0123456789abcdef","prompt":"Decision","max_input_tokens":64})", "application/json");
        if (!denied || denied->status != 401) throw std::runtime_error("protocol_self_test_failed");
        auto malformed = client.Post("/score", good, "[]", "application/json");
        if (!malformed || malformed->status != 400 || json::parse(malformed->body)["error"]["code"] != "invalid_request") throw std::runtime_error("protocol_self_test_failed");
        auto valid = client.Post("/score", good, R"({"request_id":"0123456789abcdef","prompt":"Decision","max_input_tokens":64})", "application/json");
        if (!valid || valid->status != 503 || json::parse(valid->body)["request_id"] != "0123456789abcdef") throw std::runtime_error("protocol_self_test_failed");
        auto cancel_denied = client.Post("/cancel", R"({"request_id":"0123456789abcdef"})", "application/json");
        if (!cancel_denied || cancel_denied->status != 401) throw std::runtime_error("protocol_self_test_failed");
        auto cancel = client.Post("/cancel", good, R"({"request_id":"0123456789abcdef"})", "application/json");
        if (!cancel || cancel->status != 200 || json::parse(cancel->body)["cancelled"] != true) throw std::runtime_error("protocol_self_test_failed");
        auto caps = client.Get("/capabilities", good);
        if (!caps || caps->status != 200 || json::parse(caps->body)["limits"]["max_request_bytes"] != kMaxRequestBytes) throw std::runtime_error("capabilities_self_test_failed");
        auto pre_cancel = client.Post("/score", good, R"({"request_id":"0123456789abcdef","prompt":"Decision","max_input_tokens":64})", "application/json");
        if (!pre_cancel || pre_cancel->status != 409 || json::parse(pre_cancel->body)["error"]["code"] != "request_cancelled") throw std::runtime_error("pre_cancel_self_test_failed");
        auto active = std::make_shared<RequestCancellation>();
        {
            std::lock_guard<std::mutex> guard(runtime.requests_mutex);
            runtime.active_requests["active-cancel-0123456789"] = active;
        }
        auto active_cancel = client.Post("/cancel", good, R"({"request_id":"active-cancel-0123456789"})", "application/json");
        if (!active_cancel || active_cancel->status != 200 || !active->cancelled.load()) throw std::runtime_error("active_cancel_self_test_failed");
        for (const auto & malformed_body : {std::string("{"), std::string("null"), std::string(33, '[') + "0" + std::string(33, ']')}) {
            for (const auto & route : {"/score", "/cancel"}) {
                auto invalid = client.Post(route, good, malformed_body, "application/json");
                if (!invalid || invalid->status != 400) throw std::runtime_error("json_self_test_failed");
            }
        }
        std::string score_body = R"({"request_id":"0123456789abcdef","prompt":"Decision","max_input_tokens":64,"nested":)";
        auto depth32 = client.Post("/score", good, score_body + std::string(31,'[') + "0" + std::string(31,']') + "}", "application/json");
        auto depth33 = client.Post("/score", good, score_body + std::string(32,'[') + "0" + std::string(32,']') + "}", "application/json");
        if (!depth32 || depth32->status != 503 || !depth33 || depth33->status != 400) throw std::runtime_error("depth_self_test_failed");
        score_body = R"({"request_id":"0123456789abcdef","prompt":"Decision","max_input_tokens":64})";
        score_body.resize(kMaxRequestBytes, ' ');
        auto score_limit = client.Post("/score", good, score_body, "application/json");
        if (!score_limit || score_limit->status != 503) throw std::runtime_error("score_limit_self_test_failed");
        std::string cancel_body = R"({"request_id":"cancel-limit-0123456789"})";
        cancel_body.resize(kMaxCancelBytes, ' ');
        auto cancel_limit = client.Post("/cancel", good, cancel_body, "application/json");
        auto cancel_over = client.Post("/cancel", good, cancel_body + " ", "application/json");
        if (!cancel_limit || cancel_limit->status != 200 || !cancel_over || cancel_over->status != 413) throw std::runtime_error("cancel_limit_self_test_failed");

        auto raw_status = [port](const std::string & request) {
            struct SocketOwner {
                decltype(::socket(AF_INET, SOCK_STREAM, 0)) value;
                ~SocketOwner() {
#if defined(_WIN32)
                    ::closesocket(value);
#else
                    ::close(value);
#endif
                }
            } socket{::socket(AF_INET, SOCK_STREAM, 0)};
            sockaddr_in address{};
            address.sin_family = AF_INET;
            address.sin_port = htons(static_cast<uint16_t>(port));
            address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
            if (::connect(socket.value, reinterpret_cast<const sockaddr *>(&address), sizeof(address)) != 0) throw std::runtime_error("raw_connect_self_test_failed");
            size_t sent = 0;
            while (sent < request.size()) {
                const auto n = ::send(socket.value, request.data()+sent, static_cast<int>(request.size()-sent), 0);
                if (n <= 0) throw std::runtime_error("raw_send_self_test_failed");
                sent += static_cast<size_t>(n);
            }
            fd_set readable;
            FD_ZERO(&readable);
            FD_SET(socket.value, &readable);
            timeval timeout{2,0};
            if (::select(static_cast<int>(socket.value)+1, &readable, nullptr, nullptr, &timeout) <= 0) throw std::runtime_error("body_wait_self_test_failed");
            char buffer[1024];
            const auto n = ::recv(socket.value, buffer, sizeof(buffer), 0);
            return n > 0 ? std::string(buffer, static_cast<size_t>(n)) : std::string();
        };
        auto require_status = [&raw_status](const std::string & request, const char * status) {
            if (raw_status(request).find(std::string("HTTP/1.1 ")+status) != 0) throw std::runtime_error("early_http_self_test_failed");
        };
        const std::string headers = "POST /score HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n";
        require_status(headers + "Content-Length: 41943040\r\n\r\n", "401");
        require_status(headers + "Content-Length: 41943040\r\nExpect: 100-continue\r\n\r\n", "401");
        require_status(headers + "Authorization: Bearer " + configured + "\r\nContent-Length: 41943041\r\n\r\n", "413");
        const std::string chunk_headers = "POST /cancel HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\nAuthorization: Bearer " + configured + "\r\nTransfer-Encoding: chunked\r\n\r\n";
        require_status(chunk_headers + "1000\r\n" + cancel_body + "\r\n0\r\n\r\n", "200");
        require_status(chunk_headers + "1001\r\n" + cancel_body + " \r\n0\r\n\r\n", "413");
        passed = true;
    } catch (...) {
        server.stop();
        if (worker.joinable()) worker.join();
        throw;
    }
    server.stop();
    if (worker.joinable()) worker.join();
    if (!passed) throw std::runtime_error("protocol_self_test_failed");
    std::cout << json{{"self_test", "native-runtime-v1"}, {"http_loopback", true},
        {"scheduler_preflight", {{"denial_precedes_compute", true}, {"allow_path_computes", true}}},
        {"checks", {"auth-before-body", "health", "capabilities", "invalid-score", "request-id-echo", "cancel-route", "active-cancel", "pre-cancel", "json-depth", "score-limit", "cancel-limit-chunked"}}}.dump() << std::endl;
    return 0;
}

void self_test_scheduler_preflight() {
    ggml_backend_t backend = ggml_backend_cpu_init();
    if (!backend) throw std::runtime_error("scheduler_preflight_backend_unavailable");
    ggml_backend_buffer_type_t buffer_type = ggml_backend_get_default_buffer_type(backend);
    ggml_backend_t backends[] = {backend};
    ggml_backend_buffer_type_t buffer_types[] = {buffer_type};
    ggml_backend_sched_t scheduler = ggml_backend_sched_new(backends, buffer_types, 1, 8, false, true);
    ggml_context * ctx = nullptr;
    ggml_context * input_ctx = nullptr;
    ggml_backend_buffer_t input_buffer = nullptr;
    auto cleanup = [&]() {
        if (scheduler) ggml_backend_sched_free(scheduler);
        if (ctx) ggml_free(ctx);
        if (input_buffer) ggml_backend_buffer_free(input_buffer);
        if (input_ctx) ggml_free(input_ctx);
        if (backend) ggml_backend_free(backend);
    };
    if (!scheduler) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_init_failed");
    }

    ggml_init_params params{};
    params.mem_size = 1024 * 1024;
    params.no_alloc = true;
    input_ctx = ggml_init(params);
    if (!input_ctx) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_init_failed");
    }
    ggml_tensor * left = ggml_new_tensor_1d(input_ctx, GGML_TYPE_F32, 1);
    ggml_tensor * right = ggml_new_tensor_1d(input_ctx, GGML_TYPE_F32, 1);
    ggml_set_input(left);
    ggml_set_input(right);
    input_buffer = ggml_backend_alloc_ctx_tensors(input_ctx, backend);
    if (!input_buffer) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_alloc_failed");
    }
    ctx = ggml_init(params);
    if (!ctx) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_init_failed");
    }

    ggml_tensor * sum = ggml_add(ctx, left, right);
    ggml_set_output(sum);
    ggml_cgraph * graph = ggml_new_graph(ctx);
    ggml_build_forward_expand(graph, sum);
    PreflightProbe probe;
    ggml_backend_sched_set_preflight_callback(scheduler, preflight_probe_callback, &probe);
    if (!ggml_backend_sched_alloc_graph(scheduler, graph)) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_alloc_failed");
    }

    const float left_value = 2.0f;
    const float right_value = 3.0f;
    const float sentinel = 77.0f;
    ggml_backend_tensor_set(left, &left_value, 0, sizeof(left_value));
    ggml_backend_tensor_set(right, &right_value, 0, sizeof(right_value));
    ggml_backend_tensor_set(sum, &sentinel, 0, sizeof(sentinel));

    const ggml_status rejected = ggml_backend_sched_graph_compute(scheduler, graph);
    float actual = 0.0f;
    ggml_backend_tensor_get(sum, &actual, 0, sizeof(actual));
    if (rejected != GGML_STATUS_ABORTED || probe.calls == 0 || actual != sentinel) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_rejection_failed");
    }

    probe.allow = true;
    if (ggml_backend_sched_graph_compute(scheduler, graph) != GGML_STATUS_SUCCESS) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_allow_failed");
    }
    ggml_backend_tensor_get(sum, &actual, 0, sizeof(actual));
    if (actual != 5.0f) {
        cleanup();
        throw std::runtime_error("scheduler_preflight_compute_failed");
    }
    cleanup();
}
} // namespace

int main(int argc, char ** argv) {
    try {
        Config config = parse_args(argc, argv);
        if (config.discover) { std::cout << discover_json(config.model_path).dump() << std::endl; return 0; }
    if (config.self_test) {
        self_test_scheduler_preflight();
        return self_test_protocol();
    }
        Runtime runtime(std::move(config));
        runtime.initialize();
        runtime.serve();
        return 0;
    } catch (const CandidateTokenError &) {
        std::cerr << "AURAGO_DECISIONS_STARTUP_ERROR code=runtime_start_failed\n";
    } catch (const std::exception & e) {
        const std::string message = e.what();
        // Emit only the manager's stable allowlist; never print arguments,
        // filesystem paths, request payloads, or third-party error strings.
        static const std::set<std::string> invalid_configuration = {
            "artifact_unreadable", "artifact_read_failed", "token_file_unreadable", "token_file_invalid",
            "select_exactly_one_device_or_allow_cpu", "runtime_arguments_incomplete", "cpu_consent_requires_cpu_build",
            "cpu_build_requires_explicit_cpu_consent", "cpu_offload_requires_selected_gpu", "gpu_layers_requires_cpu_offload_consent",
            "gpu_layer_count_invalid", "unknown_runtime_argument", "missing_value_for_--model", "missing_value_for_--model-revision",
            "missing_value_for_--model-sha256", "missing_value_for_--mmproj", "missing_value_for_--mmproj-revision",
            "missing_value_for_--mmproj-sha256", "missing_value_for_--device", "missing_value_for_--allow-cpu",
            "missing_value_for_--allow-cpu-offload", "missing_value_for_--gpu-layers", "missing_value_for_--bind",
            "missing_value_for_--port", "missing_value_for_--token-file"
        };
        static const std::set<std::string> device_unavailable = {
            "selected_device_not_found", "selected_device_not_cpu", "selected_device_not_accelerator", "selected_device_backend_mismatch"
        };
        static const std::set<std::string> offload_unavailable = {
            "context_init_failed", "attestation_warmup_failed", "attestation_tokenize_failed", "attestation_context_invalid",
            "attestation_inference_failed", "device_attestation_incomplete", "http_bind_failed", "http_listen_failed",
            "strict_device_preflight_rejected", "full_gpu_offload_not_attested"
        };
        std::string code = "runtime_start_failed";
        if (invalid_configuration.count(message)) code = "invalid_configuration";
        else if (device_unavailable.count(message)) code = "device_unavailable";
        else if (offload_unavailable.count(message)) code = "offload_unavailable";
        else if (message == "model_pin_mismatch" || message == "projector_pin_mismatch" ||
                 message == "model_load_failed" || message == "projector_load_failed" || message == "unauthorized_cpu_compute") code = message;
        else if (message == "image_marker_unavailable") code = "projector_load_failed";
        std::cerr << "AURAGO_DECISIONS_STARTUP_ERROR code=" << code << "\n";
    }
    return 2;
}
