"""Exercise the actual pinned plugin's RF gain block against a fake RTL device."""
import pathlib
import subprocess
import tempfile
import unittest


class TunerGainContracts(unittest.TestCase):
    def test_automatic_manual_and_gain_only_transitions(self):
        source = pathlib.Path('/build/sdrangel/plugins/samplesource/rtlsdr/rtlsdrthread.cpp')
        if not source.exists():
            self.skipTest('runs in the native receiver image')
        text = source.read_text()
        start = text.index('    if ((settingsKeys.contains("gain")')
        end = text.index('    if ((settingsKeys.contains("biasTee")', start)
        block = text[start:end]
        # Compile upstream implementation, rather than a duplicate of the gain
        # logic. A digital-only AGC implementation fails these USB call checks.
        harness = r'''
#include <cassert>
#include <initializer_list>
#include <string>
#include <utility>
#include <vector>
using Event = std::pair<std::string, int>;
std::vector<Event> calls;
int rtlsdr_set_tuner_gain_mode(void*, int value) { calls.emplace_back("mode", value); return 0; }
int rtlsdr_set_tuner_gain(void*, int value) { calls.emplace_back("gain", value); return 0; }
void qCritical(const char*, ...) {}
void qDebug(const char*, ...) {}
struct Settings { bool m_agc; int m_gain; };
struct Keys {
    std::string values;
    bool contains(const char* key) const { return values.find(key) != std::string::npos; }
};
void apply(Settings settings, Settings m_settings, Keys settingsKeys, bool force) {
    void* m_dev = nullptr;
''' + block + r'''
}
void check(Settings desired, Settings previous, Keys keys, bool force,
           std::initializer_list<Event> expected) {
    calls.clear(); apply(desired, previous, keys, force);
    assert(calls == std::vector<Event>(expected));
}
int main() {
    check({true, 0}, {true, 0}, {""}, true, {{"mode", 0}});
    check({true, 0}, {false, 0}, {"agc"}, false, {{"mode", 0}});
    check({false, 328}, {true, 328}, {"agc"}, false, {{"mode", 1}, {"gain", 328}});
    check({false, 402}, {false, 328}, {"gain"}, false, {{"mode", 1}, {"gain", 402}});
    check({true, 402}, {true, 328}, {"gain"}, false, {{"mode", 0}});
    check({false, 328}, {false, 328}, {"gain"}, false, {});
}
'''
        with tempfile.TemporaryDirectory() as directory:
            cpp = pathlib.Path(directory) / 'tuner.cpp'
            binary = pathlib.Path(directory) / 'tuner'
            cpp.write_text(harness)
            subprocess.run(['c++', '-std=c++17', '-o', str(binary), str(cpp)], check=True, timeout=30)
            subprocess.run([str(binary)], check=True, timeout=5)


if __name__ == '__main__':
    unittest.main()
