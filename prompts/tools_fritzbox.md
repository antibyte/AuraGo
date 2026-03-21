---
id: "tools_fritzbox"
tags: ["conditional"]
priority: 31
conditions: ["fritzbox_system_enabled", "fritzbox_network_enabled", "fritzbox_telephony_enabled", "fritzbox_smarthome_enabled", "fritzbox_storage_enabled", "fritzbox_tv_enabled"]
---
### Fritz!Box
| Tool | Purpose |
|---|---|
| `fritzbox_system` | Hardware info, firmware version, uptime, system log, reboot |
| `fritzbox_network` | WLAN status/toggle, connected hosts, Wake-on-LAN, port forwarding |
| `fritzbox_telephony` | Call list, phonebooks, answering machine (TAM) inbox — read, download, transcribe |
| `fritzbox_smarthome` | Smart Home devices: switches, heating thermostats, lamps, templates |
| `fritzbox_storage` | NAS/storage info, FTP server, DLNA media server |
| `fritzbox_tv` | DVB-C TV channel list (cable models only) |

**Telephony / Answering Machine (TAM):**
- `get_call_list` — Recent incoming, outgoing, missed calls
- `get_tam_messages` — List answering machine messages (`tam_index` default 0)
- `download_tam_message` — Download TAM audio (WAV) to workspace (`tam_index`, `msg_index`)
- `transcribe_tam_message` — Transcribe TAM audio to text (`tam_index`, `msg_index`)
- `mark_tam_message_read` — Mark message as read (`tam_index`, `msg_index`)
- `get_phonebooks` / `get_phonebook_entries` — List phonebooks and contacts
