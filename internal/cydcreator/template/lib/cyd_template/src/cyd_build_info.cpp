// cyd_build_info.cpp - the only file that sees the build id, so a new build id
// recompiles this file alone (PlatformIO keeps every other object cached).
#include "cyd_private.h"

#if __has_include("cyd_build_id.h")
#include "cyd_build_id.h"  // written by cyd-build into include/ of the build tree
#endif
#ifndef CYD_BUILD_ID
#define CYD_BUILD_ID "dev"
#endif

namespace cyd {
namespace detail {

const char* build_id() { return CYD_BUILD_ID; }

}  // namespace detail
}  // namespace cyd
