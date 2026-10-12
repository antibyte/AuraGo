cmake_minimum_required(VERSION 3.24)
project(aurago_decisions_runtime LANGUAGES C CXX)

if(MSVC)
  # Keep the app-managed executable self-contained instead of requiring a separately
  # installed Visual C++ redistributable.
  set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>" CACHE STRING "Static MSVC runtime" FORCE)
endif()

set(AURAGO_LLAMA_CPP_COMMIT "d81235049384534c167caea52b85a694f6103d14" CACHE STRING "Pinned llama.cpp commit")
set(AURAGO_LLAMA_CPP_PATCH_SHA256 "" CACHE STRING "SHA-256 of the maintained llama.cpp patch")
set(AURAGO_RUNTIME_BACKEND "cpu" CACHE STRING "cpu, cuda, vulkan, or metal")
set_property(CACHE AURAGO_RUNTIME_BACKEND PROPERTY STRINGS cpu cuda vulkan metal)
set(AURAGO_RUNTIME_OUTPUT_DIR "${CMAKE_BINARY_DIR}/out" CACHE PATH "Runtime output directory")

if(NOT AURAGO_LLAMA_CPP_COMMIT STREQUAL "d81235049384534c167caea52b85a694f6103d14")
  message(FATAL_ERROR "llama.cpp must use the verified immutable AuraGo pin")
endif()
if(NOT AURAGO_RUNTIME_BACKEND MATCHES "^(cpu|cuda|vulkan|metal)$")
  message(FATAL_ERROR "Unsupported runtime backend: ${AURAGO_RUNTIME_BACKEND}")
endif()

set(LLAMA_BUILD_COMMON OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_TOOLS OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_SERVER OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_MTMD ON CACHE BOOL "" FORCE)
set(LLAMA_CURL OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_BORINGSSL OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_LIBRESSL OFF CACHE BOOL "" FORCE)
set(MTMD_VIDEO OFF CACHE BOOL "" FORCE)
set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
set(GGML_STATIC ON CACHE BOOL "" FORCE)
set(GGML_OPENMP OFF CACHE BOOL "" FORCE)
set(GGML_NATIVE OFF CACHE BOOL "" FORCE)
set(GGML_CUDA OFF CACHE BOOL "" FORCE)
set(GGML_VULKAN OFF CACHE BOOL "" FORCE)
set(GGML_METAL OFF CACHE BOOL "" FORCE)
if(AURAGO_RUNTIME_BACKEND STREQUAL "cuda")
  set(GGML_CUDA ON CACHE BOOL "" FORCE)
elseif(AURAGO_RUNTIME_BACKEND STREQUAL "vulkan")
  set(GGML_VULKAN ON CACHE BOOL "" FORCE)
elseif(AURAGO_RUNTIME_BACKEND STREQUAL "metal")
  set(GGML_METAL ON CACHE BOOL "" FORCE)
endif()

include(FetchContent)
FetchContent_Declare(llama_cpp
  GIT_REPOSITORY https://github.com/ggml-org/llama.cpp.git
  GIT_TAG ${AURAGO_LLAMA_CPP_COMMIT}
  GIT_SHALLOW FALSE
  GIT_PROGRESS TRUE)
FetchContent_GetProperties(llama_cpp)
if(NOT llama_cpp_POPULATED)
  # Populate only: llama.cpp must be pinned and patched before CMake configures
  # any upstream subdirectory. CMP0169 still permits the declared-details form
  # of FetchContent_Populate across our supported CMake versions.
  cmake_policy(PUSH)
  if(POLICY CMP0169)
    cmake_policy(SET CMP0169 OLD)
  endif()
  FetchContent_Populate(llama_cpp)
  cmake_policy(POP)
endif()

# AuraGo-specific hard guarantees are carried as a small patch, not a vendored fork.
find_package(Git REQUIRED)
set(AURAGO_LLAMA_PATCH "${CMAKE_CURRENT_SOURCE_DIR}/patches/strict-offload-preflight.patch")
file(SHA256 "${AURAGO_LLAMA_PATCH}" AURAGO_LLAMA_CPP_PATCH_SHA256)
execute_process(
  COMMAND "${GIT_EXECUTABLE}" -C "${llama_cpp_SOURCE_DIR}" rev-parse HEAD
  OUTPUT_VARIABLE LLAMA_CPP_HEAD OUTPUT_STRIP_TRAILING_WHITESPACE
  RESULT_VARIABLE LLAMA_CPP_REV_RESULT)
if(NOT LLAMA_CPP_REV_RESULT EQUAL 0 OR NOT LLAMA_CPP_HEAD STREQUAL AURAGO_LLAMA_CPP_COMMIT)
  message(FATAL_ERROR "llama.cpp source is not the verified immutable pin ${AURAGO_LLAMA_CPP_COMMIT}")
endif()
execute_process(
  COMMAND "${GIT_EXECUTABLE}" -C "${llama_cpp_SOURCE_DIR}" apply --check "${AURAGO_LLAMA_PATCH}"
  RESULT_VARIABLE LLAMA_PATCH_CHECK ERROR_QUIET)
if(LLAMA_PATCH_CHECK EQUAL 0)
  execute_process(
    COMMAND "${GIT_EXECUTABLE}" -C "${llama_cpp_SOURCE_DIR}" apply "${AURAGO_LLAMA_PATCH}"
    RESULT_VARIABLE LLAMA_PATCH_APPLY)
  if(NOT LLAMA_PATCH_APPLY EQUAL 0)
    message(FATAL_ERROR "Could not apply required AuraGo llama.cpp patch")
  endif()
else()
  execute_process(
    COMMAND "${GIT_EXECUTABLE}" -C "${llama_cpp_SOURCE_DIR}" apply --reverse --check "${AURAGO_LLAMA_PATCH}"
    RESULT_VARIABLE LLAMA_PATCH_REVERSE_CHECK ERROR_QUIET)
  if(NOT LLAMA_PATCH_REVERSE_CHECK EQUAL 0)
    message(FATAL_ERROR "llama.cpp source has unknown local changes; refusing to build")
  endif()
endif()
set(LLAMA_PATCH_DIFF "${CMAKE_BINARY_DIR}/llama_cpp_aura_patch.diff")
execute_process(
  COMMAND "${GIT_EXECUTABLE}" -C "${llama_cpp_SOURCE_DIR}" diff --binary
  OUTPUT_FILE "${LLAMA_PATCH_DIFF}" RESULT_VARIABLE LLAMA_DIFF_RESULT)
if(NOT LLAMA_DIFF_RESULT EQUAL 0)
  message(FATAL_ERROR "Could not inspect llama.cpp working-tree changes")
endif()
file(SHA256 "${LLAMA_PATCH_DIFF}" LLAMA_PATCH_ACTUAL_SHA256)
file(SHA256 "${AURAGO_LLAMA_PATCH}" LLAMA_PATCH_EXPECTED_SHA256)
execute_process(
  COMMAND "${GIT_EXECUTABLE}" -C "${llama_cpp_SOURCE_DIR}" status --porcelain --untracked-files=all
  OUTPUT_VARIABLE LLAMA_PATCH_STATUS OUTPUT_STRIP_TRAILING_WHITESPACE
  RESULT_VARIABLE LLAMA_PATCH_STATUS_RESULT)
set(LLAMA_PATCH_EXPECTED_STATUS " M ggml/include/ggml-backend.h\n M ggml/src/ggml-backend.cpp\n M ggml/src/ggml-vulkan/CMakeLists.txt\n M include/llama.h\n M src/llama-context.cpp\n M src/llama-context.h\n M tools/mtmd/clip.cpp\n M tools/mtmd/clip.h\n M tools/mtmd/mtmd.cpp\n M tools/mtmd/mtmd.h")
if(NOT LLAMA_PATCH_STATUS_RESULT EQUAL 0 OR NOT LLAMA_PATCH_ACTUAL_SHA256 STREQUAL LLAMA_PATCH_EXPECTED_SHA256 OR NOT LLAMA_PATCH_STATUS STREQUAL LLAMA_PATCH_EXPECTED_STATUS)
  message(FATAL_ERROR "llama.cpp source changes differ from the complete reviewed AuraGo patch")
endif()

# Only enter upstream CMake after the exact source pin and complete local patch
# have both been verified above.
add_subdirectory("${llama_cpp_SOURCE_DIR}" "${llama_cpp_BINARY_DIR}")
if(NOT TARGET cpp-httplib)
  add_subdirectory("${llama_cpp_SOURCE_DIR}/vendor/cpp-httplib" "${CMAKE_BINARY_DIR}/vendor-cpp-httplib")
endif()

add_executable(aurago-decisions-runtime src/main.cpp)
target_compile_features(aurago-decisions-runtime PRIVATE cxx_std_17)
target_include_directories(aurago-decisions-runtime PRIVATE
  "${llama_cpp_SOURCE_DIR}/include"
  "${llama_cpp_SOURCE_DIR}/ggml/include"
  "${llama_cpp_SOURCE_DIR}/src"
  "${llama_cpp_SOURCE_DIR}/tools/mtmd"
  "${llama_cpp_SOURCE_DIR}/vendor"
  "${llama_cpp_SOURCE_DIR}/vendor/cpp-httplib"
  "${llama_cpp_SOURCE_DIR}/vendor/stb"
  "${llama_cpp_SOURCE_DIR}/vendor/hash/sha256")
target_link_libraries(aurago-decisions-runtime PRIVATE llama mtmd ggml cpp-httplib vendor-hash vendor::stb vendor::nlohmann)
target_compile_definitions(aurago-decisions-runtime PRIVATE
  AURAGO_LLAMA_CPP_COMMIT="${AURAGO_LLAMA_CPP_COMMIT}"
  AURAGO_LLAMA_CPP_PATCH_SHA256="${AURAGO_LLAMA_CPP_PATCH_SHA256}"
  AURAGO_RUNTIME_BACKEND="${AURAGO_RUNTIME_BACKEND}")
if(WIN32)
  target_link_libraries(aurago-decisions-runtime PRIVATE ws2_32 bcrypt)
endif()
if(MSVC)
  target_compile_options(aurago-decisions-runtime PRIVATE /W4 /permissive-)
else()
  target_compile_options(aurago-decisions-runtime PRIVATE -Wall -Wextra -Wpedantic)
endif()
set_target_properties(aurago-decisions-runtime PROPERTIES
  RUNTIME_OUTPUT_DIRECTORY "${AURAGO_RUNTIME_OUTPUT_DIR}"
  RUNTIME_OUTPUT_DIRECTORY_DEBUG "${AURAGO_RUNTIME_OUTPUT_DIR}"
  RUNTIME_OUTPUT_DIRECTORY_RELEASE "${AURAGO_RUNTIME_OUTPUT_DIR}"
  RUNTIME_OUTPUT_DIRECTORY_RELWITHDEBINFO "${AURAGO_RUNTIME_OUTPUT_DIR}"
  RUNTIME_OUTPUT_DIRECTORY_MINSIZEREL "${AURAGO_RUNTIME_OUTPUT_DIR}")
