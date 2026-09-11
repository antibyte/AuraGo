// Exercise the exact GGML Vulkan backend used by ace-server, without CPU scheduling.
#include "ggml.h"
#include "ggml-alloc.h"
#include "ggml-backend.h"
#include "yyjson.h"
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>

int main(int argc, char ** argv) {
    ggml_backend_load_all();
    auto * doc = yyjson_mut_doc_new(nullptr);
    auto * rows = yyjson_mut_arr(doc);
    yyjson_mut_doc_set_root(doc, rows);
    for (size_t i = 0; i < ggml_backend_dev_count(); ++i) {
        auto dev = ggml_backend_dev_get(i);
        ggml_backend_dev_props props{};
        ggml_backend_dev_get_props(dev, &props);
        if (std::strncmp(props.name, "Vulkan", 6) || !props.device_id || !*props.device_id) continue;
        if (argc == 2) {
            if (std::strcmp(argv[1], props.name)) continue;
            auto backend = ggml_backend_dev_init(dev, nullptr);
            if (!backend) return 2;
            auto ctx = ggml_init({4 * 1024 * 1024, nullptr, true});
            auto a = ggml_new_tensor_2d(ctx, GGML_TYPE_F32, 16, 16);
            auto b = ggml_new_tensor_2d(ctx, GGML_TYPE_F32, 16, 16);
            auto c = ggml_mul_mat(ctx, a, b);
            auto graph = ggml_new_graph(ctx);
            ggml_build_forward_expand(graph, c);
            if (!ggml_backend_dev_supports_op(dev, c)) return 3;
            auto buffer = ggml_backend_alloc_ctx_tensors(ctx, backend);
            if (!buffer) return 4;
            float input[256], output[256];
            for (auto & x : input) x = 1;
            ggml_backend_tensor_set(a, input, 0, sizeof(input));
            ggml_backend_tensor_set(b, input, 0, sizeof(input));
            if (ggml_backend_graph_compute(backend, graph) != GGML_STATUS_SUCCESS) return 5;
            ggml_backend_synchronize(backend);
            ggml_backend_tensor_get(c, output, 0, sizeof(output));
            for (auto x : output) if (!std::isfinite(x) || std::fabs(x - 16) > 0.001f) return 6;
            ggml_backend_buffer_free(buffer);
            ggml_free(ctx);
            ggml_backend_free(backend);
            return 0;
        }
        auto row = yyjson_mut_obj(doc);
        yyjson_mut_obj_add_strcpy(doc, row, "name", props.description);
        yyjson_mut_obj_add_strcpy(doc, row, "backend_name", props.name);
        yyjson_mut_obj_add_strcpy(doc, row, "pci", props.device_id);
        yyjson_mut_obj_add_uint(doc, row, "free", props.memory_free);
        yyjson_mut_obj_add_uint(doc, row, "total", props.memory_total);
        yyjson_mut_obj_add_bool(doc, row, "shared", props.type == GGML_BACKEND_DEVICE_TYPE_IGPU);
        yyjson_mut_arr_add_val(rows, row);
    }
    if (argc == 2) return 7;
    char * json = yyjson_mut_write(doc, 0, nullptr);
    std::puts(json);
    std::free(json);
    yyjson_mut_doc_free(doc);
}
