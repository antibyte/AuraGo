# lora_regional_plans — LoRaWAN regional channel plans EU868 / US915 / AS923 / LoRaWAN-Kanalpläne EU868 / US915 / AS923

Kind: transcribed standard
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| plan | regional parameter plan name (RP002) | Name des Regionalplans (RP002) | – | no |
| role | `default_uplink` (mandatory default / join channels), `uplink_125`, `uplink_500`, `downlink_500`, `rx2` (second receive window), `beacon` (Class B) | Kanalrolle | – | no |
| channels | channel index or range | Kanalnummer oder -bereich | – | no |
| frequency | centre frequency; a list separated by ";" or a formula in n (channel index) | Mittenfrequenz; Liste mit ";" oder Formel in n (Kanalnummer) | MHz | partly |
| step | channel step for formula rows | Kanalabstand bei Formel-Zeilen | MHz | yes |
| bandwidth | LoRa channel bandwidth | LoRa-Kanalbandbreite | kHz | yes |
| data_rates | data-rate range (DRx as defined per region in RP002) | Datenraten (DRx je Region nach RP002) | – | no |
| note | remark "en\|de" | Hinweis "en\|de" | – | no |
| check | cross-check record (RP002 = LoRa Alliance spec; LoRaMac-node = Semtech reference implementation; TTN = The Things Network docs) | Quellenabgleich | – | no |

## Rows
```csv
plan,role,channels,frequency,step,bandwidth,data_rates,note,check
EU868,default_uplink,0–2,868.1; 868.3; 868.5,,125,DR0–DR5,"mandatory in every device, also the join-request channels; duty cycle < 1 %; default max EIRP +16 dBm|in jedem Gerät Pflicht, zugleich Join-Request-Kanäle; Tastgrad < 1 %; Standard-Max-EIRP +16 dBm",RP002 Tables 2/6/7 = LoRaMac-node EU868_LC1–3 = TTN docs
EU868,rx2,,869.525,,125,DR0 (SF12),default RX2; The Things Network uses SF9 (DR3) instead (network-specific)|Standard-RX2; The Things Network verwendet stattdessen SF9 (DR3) (netzspezifisch),frequency: RP002 = LoRaMac-node = TTN; DR0: RP002 = LoRaMac-node (TTN differs: SF9)
EU868,beacon,,869.525,,125,,Class B beacon and default ping slot|Class-B-Beacon und Standard-Ping-Slot,RP002 Table 2 = LoRaMac-node EU868_BEACON_CHANNEL_FREQ
US915,uplink_125,0–63,902.3 + 0.2·n,0.2,125,DR0–DR3,"64 channels 902.3–914.9 MHz; networks usually use one sub-band of 8 (e.g. TTN FSB2 = channels 8–15, 903.9–905.3 MHz)|64 Kanäle 902,3–914,9 MHz; Netze nutzen meist ein Teilband aus 8 Kanälen (z. B. TTN FSB2 = Kanäle 8–15, 903,9–905,3 MHz)",RP002 §2.5.2 = LoRaMac-node RegionUS915.c (902300000 + i·200000); FSB2 list = TTN docs
US915,uplink_500,64–71,903.0 + 1.6·(n − 64),1.6,500,DR4,"8 channels 903.0–914.2 MHz (TTN FSB2 uses channel 65 = 904.6 MHz)|8 Kanäle 903,0–914,2 MHz (TTN FSB2 nutzt Kanal 65 = 904,6 MHz)",RP002 §2.5.2 = LoRaMac-node (903000000 + k·1600000); 904.6 = TTN docs
US915,downlink_500,0–7,923.3 + 0.6·n,0.6,500,DR8–DR13,"8 downlink channels 923.3–927.5 MHz; no duty-cycle limit, 400 ms dwell time on 125 kHz uplinks, default max EIRP +30 dBm|8 Downlink-Kanäle 923,3–927,5 MHz; keine Tastgrad-Grenze, 400 ms Verweilzeit auf 125-kHz-Uplinks, Standard-Max-EIRP +30 dBm",RP002 = LoRaMac-node US915_FIRST_RX1_CHANNEL/STEPWIDTH = TTN list (923.3 … 927.5)
US915,rx2,,923.3,,500,DR8,default RX2|Standard-RX2,RP002 Table 3 = LoRaMac-node US915_RX_WND_2_FREQ/DR
AS923-1,default_uplink,0–1,923.2; 923.4,,125,DR0–DR5,"frequency offset 0; uplink dwell time 400 ms; default max EIRP +16 dBm; RX2 923.2 MHz DR2 (SF10)|Frequenzversatz 0; Uplink-Verweilzeit 400 ms; Standard-Max-EIRP +16 dBm; RX2 923,2 MHz DR2 (SF10)","RP002 §2.10.2 and Table 2 = LoRaMac-node AS923_LC1/LC2, RX2 923200000/DR_2 = TTN docs"
AS923-2,default_uplink,0–1,921.4; 921.6,,125,DR0–DR5,"AS923 with offset −1.80 MHz; RX2 921.4 MHz|AS923 mit Versatz −1,80 MHz; RX2 921,4 MHz",RP002 Table 2 = LoRaMac-node CHANNEL_PLAN_GROUP_AS923_2 offset −1.8 MHz
AS923-3,default_uplink,0–1,916.6; 916.8,,125,DR0–DR5,"AS923 with offset −6.60 MHz; RX2 916.6 MHz|AS923 mit Versatz −6,60 MHz; RX2 916,6 MHz",RP002 Table 2 = LoRaMac-node CHANNEL_PLAN_GROUP_AS923_3 offset −6.6 MHz
AS923-4,default_uplink,0–1,917.3; 917.5,,125,DR0–DR5,"AS923 with offset −5.90 MHz (used in Israel per RP002 country list); RX2 917.3 MHz|AS923 mit Versatz −5,90 MHz (laut RP002-Länderliste in Israel); RX2 917,3 MHz",RP002 Table 2 = LoRaMac-node CHANNEL_PLAN_GROUP_AS923_4 offset −5.9 MHz
```

## Sources
1. LoRa Alliance, *RP002-1.0.3 LoRaWAN® Regional Parameters* (© 2021), PDF https://lora-alliance.org/wp-content/uploads/2021/05/RP002-1.0.3-FINAL-1.pdf — Table 2 "Dynamic Channel Plans Summary" (EU868, AS923-1…4), Table 3 "Fixed Channel Plans Summary" (US915), Tables 6/7 (EU863-870 default and join channels), §2.5.2 (US902-928 channel frequencies), §2.10.2 (AS923 frequency offset AS923_FREQ_OFFSET); accessed 2026-11-02.
2. LoRa Alliance resource pages for RP002-1.0.4 and RP002-1.0.5, https://resources.lora-alliance.org/document/rp002-1-0-4-regional-parameters and https://resources.lora-alliance.org/document/rp002-1-0-5-lorawan-regional-parameters — newer versions exist; their PDFs could not be downloaded (embedded viewer), so values are from 1.0.3; accessed 2026-11-02.
3. Semtech, LoRaMac-node reference implementation (GitHub master), `src/mac/region/RegionEU868.h`, `RegionUS915.h`, `RegionUS915.c`, `RegionAS923.h`, https://github.com/Lora-net/LoRaMac-node — default channel frequencies, RX2 frequency/DR, US915 channel formulas, AS923 offsets (−1.8 / −6.6 / −5.9 MHz), default max EIRP 16 dBm (EU868, AS923); accessed 2026-11-02.
4. The Things Network, "Frequency Plans", https://www.thethingsnetwork.org/docs/lorawan/frequency-plans/ — EU868 channel list and RX2 (869.525 MHz SF9), US915 FSB2 (903.9–905.3 MHz, 904.6 MHz) and downlinks 923.3–927.5 MHz, AS920-923 (923.2/923.4 MHz); accessed 2026-11-02.
5. The Things Network, `lorawan-frequency-plans` repository, `EU_863_870.yml`, https://github.com/TheThingsNetwork/lorawan-frequency-plans — EU868 uplinks 868.1/868.3/868.5 + 867.1–867.9 MHz (network choice); accessed 2026-11-02.

## Cross-check
- Every frequency, step, bandwidth and data-rate cell is backed by RP002 (source 1) **and** the Semtech reference implementation (source 3); TTN (sources 4/5) agrees where it lists the same item. US915 formulas were checked arithmetically (a helper script that is not part of the repository: channel 63 = 914.9, channel 71 = 914.2, downlink 7 = 927.5, FSB2 = 903.9–905.3, channel 65 = 904.6 MHz; AS923-2/3/4 = 923.2/923.4 MHz + offset).
- **Disagreement:** EU868 RX2 data rate — RP002 and LoRaMac-node: DR0 (SF12/125 kHz); TTN: SF9 (DR3). **Chosen DR0** (the specification default); TTN's SF9 is noted as network-specific.
- **Disagreement (not in rows):** RP002 Table 2 shows "Duty Cycle < 1 %" for AS923, but LoRaMac-node sets `AS923_DUTY_CYCLE_ENABLED 0` (the limit is country-specific). Duty cycle is therefore not stated for AS923.
- EU868 extra channels (867.1–867.9 MHz) are a TTN network choice, not a spec default; not in the rows.
- **Version:** values are from RP002-1.0.3 (2021). RP002-1.0.4 (2022) and RP002-1.0.5 exist; I could not read them. The default channels shown here are the long-standing plan definitions (also unchanged in the current LoRaMac-node master), but the implementer should re-check against 1.0.5 if available.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. The national radio regulations (power, duty cycle, dwell time) always take precedence over the LoRaWAN plan.
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Regeln. Die nationalen Funkvorschriften (Leistung, Tastgrad, Verweilzeit) haben immer Vorrang vor dem LoRaWAN-Plan.
- en: Network servers can add up to 16 channels in EU868 and AS923 (CFList); only the default channels are guaranteed.
- de: Netzwerkserver können in EU868 und AS923 bis zu 16 Kanäle hinzufügen (CFList); garantiert sind nur die Standardkanäle.

## Suggested finder / lookups
- US915 channel number n → frequency (formula rows); frequency → channel number (inverse).
- Plan → default join channels and RX2 settings.
