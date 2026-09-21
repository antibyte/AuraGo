"""Build only the receive plugins used by AuraGo, from a pinned SDRangel tree."""
import pathlib
import re
import subprocess

allowed = {
    "ENABLE_RTLSDR", "ENABLE_CHANNELRX", "ENABLE_CHANNELRX_DEMODAM",
    "ENABLE_CHANNELRX_DEMODBFM", "ENABLE_CHANNELRX_DEMODNFM", "ENABLE_CHANNELRX_DEMODSSB",
}
options = re.findall(r"option\((ENABLE_[\w.]+)\s", pathlib.Path("CMakeLists.txt").read_text())
subprocess.run([
    "cmake", "-S", ".", "-B", "out", "-DCMAKE_BUILD_TYPE=Release",
    "-DCMAKE_INSTALL_PREFIX=/opt/sdrangel", "-DBUILD_GUI=OFF", "-DBUILD_SERVER=ON",
    "-DBUILD_BENCH=OFF", "-DRX_SAMPLE_24BIT=OFF",
    *[f"-D{option}={'ON' if option in allowed else 'OFF'}" for option in options],
], check=True)
