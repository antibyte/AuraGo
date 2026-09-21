"""Contracts for the pinned decoder APIs; no USB device is required."""
import queue
import threading
import unittest
from unittest.mock import patch, MagicMock

import receiver


class ReceiverContracts(unittest.TestCase):
    def test_analog_retuning_keeps_decoder_alive(self):
        radio = receiver.Receiver.__new__(receiver.Receiver)
        radio.lock = threading.RLock()
        radio.index, radio.mode = 0, "wfm"
        radio.meta = {"minimum_hz": 24000000, "maximum_hz": 1766000000}
        radio.children, radio.decoder = [MagicMock()], MagicMock()
        radio.children[0].poll.return_value = radio.decoder.poll.return_value = None
        tuning = {"mode": "wfm", "frequency_hz": 101100000}
        with patch.object(radio, "configure_analog") as configure, patch.object(radio, "stop") as stop:
            radio.tune(tuning)
            configure.assert_called_once_with(tuning)
            stop.assert_not_called()
        self.assertEqual(radio.meta["center_hz"], 101100000)

    def test_slow_browser_does_not_mark_capture_as_dropped(self):
        hub = receiver.Hub()
        slow, capture = hub.subscribe(), hub.subscribe()
        for i in range(150):
            hub.publish(b"pcm")
            self.assertEqual(capture.get_nowait(), b"pcm")
        self.assertEqual(slow.qsize(), 100)
        self.assertEqual(slow.dropped, 50)
        self.assertEqual(capture.dropped, 0)
        hub.remove(slow)
        hub.remove(capture)

    def test_dab_service_id_matches_real_welle_hex_format(self):
        radio = receiver.Receiver.__new__(receiver.Receiver)
        radio.generation = 1
        radio.last_audio = float("inf")
        mux = {"services": [{"sid": "0xd210", "label": {"label": "Fixture"},
                "components": [{"transportmode": "audio"}], "dls": {"label": "News"}}],
               "demodulator": {"snr": 24}}
        radio.tuning = {"dab_block": "5C", "service_id": "0xd210"}
        radio.meta = {}
        with patch.object(receiver, "request", return_value=mux), \
             patch.object(receiver.subprocess, "Popen") as process, \
             patch.object(receiver.threading, "Thread"):
            process.return_value.poll.return_value = None
            radio.start_dab(radio.tuning, True)
            args = process.call_args.args[0]
            self.assertIn("http://127.0.0.1:7979/stream/0xd210", args)
            stations = radio.dab_stations()
        self.assertEqual(stations[0]["tuning"]["service_id"], "0xd210")
        self.assertEqual(radio.meta["label"], "Fixture")
        self.assertEqual(radio.meta["text"], "News")

    def test_dab_data_service_is_not_an_audio_station(self):
        radio = receiver.Receiver.__new__(receiver.Receiver)
        radio.tuning = {"dab_block": "5C"}
        radio.meta = {}
        with patch.object(receiver, "request", return_value={"services": [
            {"sid": "0xbeef", "components": [{"transportmode": "data"}]}]}):
            self.assertEqual(radio.dab_stations(), [])


if __name__ == "__main__":
    unittest.main()
