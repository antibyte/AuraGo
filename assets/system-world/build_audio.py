"""System World ambience recordings: cut, loop, level and encode the CC0 sources.

    python assets/system-world/build_audio.py --sources <dir> [--only <id>,<id>]
    python assets/system-world/build_audio.py --check

Reads audio-sources.json (provenance and the cut of each recording). With --sources it expects
each download at <dir>/<id>/<source_file>, verifies its sha256, and writes one seamless mono
loop per recording to ui/3d/system-world/audio/<id>.ogg plus manifest.json:

  * the cut starts at `start` and is `loop_seconds` long;
  * the first `seam_seconds` are an equal-power crossfade from the second after the cut, so the
    end of the loop runs into its start without a click;
  * a high-pass removes rumble below `highpass` Hz;
  * the loop is levelled to the target integrated loudness and encoded with libvorbis.

Two kinds are not loops. `"kind": "pass"` is one event played once (a tram passing): `seconds`
from `start`, faded over `fade_in` and `fade_out`, levelled to `peak_db`. `"kind": "slices"` is
a row of single short events (footfalls): each start in `slices` gives `slice_seconds` of sound,
levelled to `peak_db` and placed on a grid of `slot` seconds, so slice k starts at k * slot; the
manifest carries `slot` and `count` for the player.

--only rebuilds the named recordings and keeps the shipped files and manifest entries of the
others, so re-cutting one loop does not re-encode the rest.

--check needs no sources: the shipped files must match the manifest (bytes, sha256), the
manifest must list exactly the recordings of audio-sources.json, every licence must be
CC0-1.0 and the total must stay within the budget. The shipped manifest carries no URLs.

Needs ffmpeg with libvorbis on PATH. Encoder output can differ between ffmpeg versions, so a
rebuild rewrites the manifest's hashes.
"""
import argparse
import hashlib
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
SOURCES = HERE / 'audio-sources.json'
OUT = ROOT / 'ui' / '3d' / 'system-world' / 'audio'


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(args):
    done = subprocess.run(args, capture_output=True, text=True, encoding='utf-8', errors='replace')
    if done.returncode:
        sys.exit('ffmpeg failed:\n' + done.stderr[-2000:])
    return done.stderr


def loudness(path):
    """Integrated loudness (LUFS) and true peak (dBTP) from ffmpeg's EBU R 128 meter."""
    report = run(['ffmpeg', '-hide_banner', '-nostats', '-i', str(path), '-af', 'ebur128=peak=true', '-f', 'null', '-'])
    summary = report[report.rfind('Summary:'):]
    integrated = float(re.search(r'I:\s+(-?[\d.]+) LUFS', summary).group(1))
    peak = float(re.search(r'Peak:\s+(-?[\d.]+) dBFS', summary).group(1))
    return integrated, peak


def peak_db(path):
    """Sample peak (dBFS); works on clips too short for the loudness meter."""
    report = run(['ffmpeg', '-hide_banner', '-nostats', '-i', str(path), '-af', 'volumedetect', '-f', 'null', '-'])
    return float(re.search(r'max_volume:\s+(-?[\d.]+) dB', report).group(1))


def build(sources_dir, only=None):
    spec = json.loads(SOURCES.read_text(encoding='utf-8'))
    target = spec['target']
    seam, rate = target['seam_seconds'], target['rate']
    OUT.mkdir(parents=True, exist_ok=True)
    entries = []
    keep = {}
    if only:
        unknown = only - {rec['id'] for rec in spec['recordings']}
        if unknown:
            sys.exit(f"--only names recordings that audio-sources.json does not list: {', '.join(sorted(unknown))}")
        current = json.loads((OUT / 'manifest.json').read_text(encoding='utf-8'))
        keep = {e['id']: e for e in current['recordings']}
    with tempfile.TemporaryDirectory() as scratch:
        for rec in spec['recordings']:
            if only and rec['id'] not in only:
                if rec['id'] not in keep:
                    sys.exit(f"{rec['id']}: not in the shipped manifest; rebuild it too")
                entries.append(keep[rec['id']])
                continue
            source = Path(sources_dir) / rec['id'] / rec['source_file']
            if not source.is_file():
                sys.exit(f'missing source: {source}')
            if sha256(source) != rec['source_sha256']:
                sys.exit(f"{source}: sha256 differs from audio-sources.json")
            kind = rec.get('kind', 'loop')
            raw = Path(scratch) / f"{rec['id']}.wav"
            clean = f"aformat=channel_layouts=mono,aresample={rate},highpass=f={rec['highpass']}"
            extra = {}
            if kind == 'loop':
                seconds = rec['loop_seconds']
                # head fades in over the seam while the second after the cut fades out: out[0] == source[loop].
                graph = (
                    f"[0:a]{clean},asplit=3[a][b][c];"
                    f"[a]atrim=0:{seam},asetpts=PTS-STARTPTS,afade=t=in:d={seam}:curve=qsin[head];"
                    f"[b]atrim={seconds}:{seconds + seam},asetpts=PTS-STARTPTS,afade=t=out:d={seam}:curve=qsin[tail];"
                    f"[head][tail]amix=inputs=2:normalize=0[x];"
                    f"[c]atrim={seam}:{seconds},asetpts=PTS-STARTPTS[mid];"
                    f"[x][mid]concat=n=2:v=0:a=1[out]"
                )
                run(['ffmpeg', '-hide_banner', '-nostats', '-y', '-ss', str(rec['start']), '-t', str(seconds + seam), '-i', str(source),
                     '-filter_complex', graph, '-map', '[out]', '-c:a', 'pcm_f32le', str(raw)])
                measured, peak = loudness(raw)
                gain = target['lufs'] - measured
                if peak + gain > -1.0:
                    gain = -1.0 - peak
            elif kind == 'pass':
                # One event from start to end (a vehicle passing): faded in and out, played once, never looped.
                seconds = rec['seconds']
                run(['ffmpeg', '-hide_banner', '-nostats', '-y', '-ss', str(rec['start']), '-t', str(seconds), '-i', str(source), '-af',
                     f"{clean},afade=t=in:d={rec['fade_in']},afade=t=out:st={seconds - rec['fade_out']}:d={rec['fade_out']}",
                     '-c:a', 'pcm_f32le', str(raw)])
                gain = rec['peak_db'] - peak_db(raw)
                extra = {'kind': kind}
            elif kind == 'slices':
                # Single short events (footfalls) on a fixed grid: slice k starts at k * slot, each at the same peak.
                slot, length, parts = rec['slot'], rec['slice_seconds'], []
                for index, start in enumerate(rec['slices']):
                    cut, part = Path(scratch) / f"{rec['id']}-{index}-cut.wav", Path(scratch) / f"{rec['id']}-{index}.wav"
                    run(['ffmpeg', '-hide_banner', '-nostats', '-y', '-ss', str(start), '-t', str(length), '-i', str(source), '-af',
                         f"{clean},afade=t=in:d=0.003,afade=t=out:st={length - .04}:d=0.04", '-c:a', 'pcm_f32le', str(cut)])
                    run(['ffmpeg', '-hide_banner', '-nostats', '-y', '-i', str(cut), '-af',
                         f"volume={rec['peak_db'] - peak_db(cut):.2f}dB,apad=whole_dur={slot}", '-c:a', 'pcm_f32le', str(part)])
                    parts.append(part)
                inputs = [arg for part in parts for arg in ('-i', str(part))]
                run(['ffmpeg', '-hide_banner', '-nostats', '-y', *inputs, '-filter_complex',
                     ''.join(f'[{i}:a]' for i in range(len(parts))) + f'concat=n={len(parts)}:v=0:a=1[out]', '-map', '[out]',
                     '-c:a', 'pcm_f32le', str(raw)])
                seconds, gain = round(slot * len(parts), 3), 0.0
                extra = {'kind': kind, 'slot': slot, 'count': len(parts)}
            else:
                sys.exit(f"{rec['id']}: unknown kind {kind}")
            final = OUT / f"{rec['id']}.ogg"
            run(['ffmpeg', '-hide_banner', '-nostats', '-y', '-i', str(raw), '-af', f'volume={gain:.2f}dB', '-map_metadata', '-1',
                 '-fflags', '+bitexact', '-flags:a', '+bitexact', '-c:a', target['codec'], '-q:a', str(target['quality']), str(final)])
            level, top = loudness(final)
            entries.append({'id': rec['id'], 'file': final.name, 'bytes': final.stat().st_size, 'sha256': sha256(final),
                            'seconds': seconds, 'lufs': round(level, 1), 'true_peak_db': round(top, 1), **extra,
                            'title': rec['title'], 'author': rec['author'], 'licence': rec['licence']})
            print(f"BUILT {rec['id']:7s} {final.stat().st_size:7d} bytes  {level:6.1f} LUFS  {top:5.1f} dBTP  gain {gain:+.1f} dB")
    manifest = {'schema_version': 1, 'licence': 'CC0-1.0', 'channels': target['channels'], 'rate': rate,
                'total_bytes': sum(e['bytes'] for e in entries), 'recordings': entries}
    (OUT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8', newline='\n')
    check()


def check():
    spec = json.loads(SOURCES.read_text(encoding='utf-8'))
    manifest = json.loads((OUT / 'manifest.json').read_text(encoding='utf-8'))
    ids = [r['id'] for r in spec['recordings']]
    assert [e['id'] for e in manifest['recordings']] == ids, 'manifest and audio-sources.json list different recordings'
    assert all(r['licence'] == 'CC0-1.0' for r in spec['recordings']), 'every source must be CC0-1.0'
    assert 'http' not in (OUT / 'manifest.json').read_text(encoding='utf-8'), 'the shipped manifest carries no URLs'
    total = 0
    for entry in manifest['recordings']:
        path = OUT / entry['file']
        assert path.is_file(), f'missing {path}'
        assert path.stat().st_size == entry['bytes'], f"{entry['file']}: size differs from the manifest"
        assert sha256(path) == entry['sha256'], f"{entry['file']}: sha256 differs from the manifest"
        assert entry['licence'] == 'CC0-1.0', f"{entry['file']}: licence"
        total += entry['bytes']
    shipped = sorted(p.name for p in OUT.iterdir() if p.suffix == '.ogg')
    assert shipped == sorted(e['file'] for e in manifest['recordings']), f'unlisted audio files: {shipped}'
    assert total == manifest['total_bytes'], 'total_bytes differs'
    assert total <= spec['target']['budget_bytes'], f"audio budget exceeded: {total} > {spec['target']['budget_bytes']}"
    print(f"System World audio verified: {len(shipped)} CC0 recordings, {total} bytes of {spec['target']['budget_bytes']}")


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--sources', help='directory with one folder per recording id')
    parser.add_argument('--only', help='comma-separated recording ids to rebuild; the others keep their shipped files and manifest entries')
    parser.add_argument('--check', action='store_true')
    options = parser.parse_args()
    if options.sources:
        build(options.sources, set(options.only.split(',')) if options.only else None)
    elif options.check:
        check()
    else:
        parser.error('give --sources <dir> or --check')
