package videostudio

import "fmt"

// Upgrade converts a stored or submitted project to the current model. Version 1 projects use fixed
// 30 fps frames, so their timing is kept as is; assets gain a microsecond duration derived from frames.
// The input is not modified.
func Upgrade(project Project) (Project, error) {
	switch project.Version {
	case CurrentVersion:
		return project, nil
	case 1:
		if project.FPS != FramesPerSecond || project.NTSC {
			return project, fmt.Errorf("version 1 projects must use %d fps", FramesPerSecond)
		}
		project.Version = CurrentVersion
		project.Assets = append([]Asset(nil), project.Assets...)
		for i := range project.Assets {
			if project.Assets[i].DurationUs == 0 && project.Assets[i].DurationFrames > 0 && project.Assets[i].Kind != AssetImage {
				project.Assets[i].DurationUs = int64(project.Assets[i].DurationFrames) * 1_000_000 / FramesPerSecond
			}
		}
		return project, nil
	default:
		return project, fmt.Errorf("unsupported project version %d", project.Version)
	}
}
