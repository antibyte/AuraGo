package videostudio

import "fmt"

// Upgrade converts a stored or submitted project to the current model, one version step at a time:
//   - 1 -> 2: version 1 projects use fixed 30 fps frames, so their timing is kept as is; assets gain a microsecond
//     duration derived from frames (BackfillDurationUs).
//   - 2 -> 3: no data changes. Version 3 only allows optional phase 2 fields (keyframes, ramps, effects, ...);
//     older builds accept only version 2 and so refuse a document whose fields they would drop on save.
//
// Any other version is an error. The input is not modified.
func Upgrade(project Project) (Project, error) {
	if project.Version < 1 || project.Version > CurrentVersion {
		return project, fmt.Errorf("unsupported project version %d", project.Version)
	}
	for project.Version < CurrentVersion {
		switch project.Version {
		case 1:
			if project.FPS != FramesPerSecond || project.NTSC {
				return project, fmt.Errorf("version 1 projects must use %d fps", FramesPerSecond)
			}
			project.Assets = append([]Asset(nil), project.Assets...)
			for i := range project.Assets {
				project.Assets[i] = BackfillDurationUs(project.Assets[i])
			}
			project.Version = 2
		case 2:
			project.Version = 3
		default:
			return project, fmt.Errorf("unsupported project version %d", project.Version)
		}
	}
	return project, nil
}
