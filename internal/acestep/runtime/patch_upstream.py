"""Apply narrow, revision-bound startup hooks; fail when upstream context drifts."""
from pathlib import Path
import sys

def patch(root):
    path = root / "acestep/api/startup_model_init.py"
    source = path.read_text()
    old = "gpu_config = get_gpu_config()"
    assert source.count(old) == 1
    source = source.replace(old, "from aurago_runtime import selected_gpu_config\n    gpu_config = selected_gpu_config()")
    old = "        offload_dit_to_cpu=offload_dit_to_cpu,\n"
    assert source.count(old) == 3
    source = source.replace(old, old + '        quantization=os.getenv("AURAGO_QUANTIZATION") or None,\n')
    old = '        "offload_dit_to_cpu": offload_dit_to_cpu,\n'
    assert source.count(old) == 1
    source = source.replace(old, old + '        "quantization": os.getenv("AURAGO_QUANTIZATION") or None,\n')
    compile(source, str(path), "exec")
    path.write_text(source)
    path = root / "acestep/api/model_download.py"
    source = path.read_text()
    old = 'def ensure_model_downloaded(model_name: str, checkpoint_dir: str) -> str:\n'
    assert source.count(old) == 1
    source = source.replace(old, old + '    from aurago_runtime import ensure_model\n    return ensure_model(model_name, checkpoint_dir)\n')
    compile(source, str(path), "exec")
    path.write_text(source)

if __name__ == "__main__": patch(Path(sys.argv[1]))
