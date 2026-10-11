package cydcreator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// loadGenerateInput reads the layout, the app files and the asset originals
// of a project tree. Assets whose file is missing (or not a regular file) are
// left out (the generator reports them); an entry whose file breaks the asset
// file rule is refused before anything is read.
func loadGenerateInput(row projectRow, root *os.Root) (GenerateInput, error) {
	p, err := readLayout(root)
	if err != nil {
		return GenerateInput{}, err
	}
	appFiles, err := readAppFiles(root)
	if err != nil {
		return GenerateInput{}, err
	}
	assets := make(map[string][]byte, len(p.Assets))
	for _, a := range p.Assets {
		if !assetFileRegex.MatchString(a.File) {
			return GenerateInput{}, assetFileError(a.ID)
		}
		data, err := rootReadFile(root, a.File, MaxAssetUploadBytes)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegularFile) {
			continue
		}
		if errors.Is(err, errFileTooLarge) {
			return GenerateInput{}, NewError(CodeAssetTooLarge, fmt.Sprintf("asset %s: the image file is larger than %d bytes", a.ID, MaxAssetUploadBytes),
				map[string]any{"asset": a.ID, "reason": "file_size", "max": MaxAssetUploadBytes})
		}
		if err != nil {
			return GenerateInput{}, fmt.Errorf("read asset %s: %w", a.File, err)
		}
		assets[a.ID] = data
	}
	return GenerateInput{Project: p, ProjectID: row.ID, AppFiles: appFiles, Assets: assets}, nil
}

// runGenerate calls the generator. A panic or an untyped failure becomes
// *Error CYD_GENERATOR_ERROR (a generator bug, never the user's fault).
func (s *Service) runGenerate(in GenerateInput) (out GenerateOutput, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("CYD Creator generator panicked", "project_id", in.ProjectID, "panic", fmt.Sprint(r))
			out, err = GenerateOutput{}, NewError(CodeGeneratorError, "the code generator failed", nil)
		}
	}()
	out, err = s.generate(in)
	if err != nil {
		var typed *Error
		if errors.As(err, &typed) {
			return out, err
		}
		s.logger.Warn("CYD Creator generator failed", "project_id", in.ProjectID, "error", err)
		return GenerateOutput{}, NewError(CodeGeneratorError, "the code generator failed", nil)
	}
	if len(out.Files) == 0 {
		return GenerateOutput{}, NewError(CodeGeneratorError, "the code generator returned no files", nil)
	}
	return out, nil
}

func (s *Service) generateFromRoot(row projectRow, root *os.Root) (GenerateOutput, error) {
	in, err := loadGenerateInput(row, root)
	if err != nil {
		return GenerateOutput{}, s.internalError("generate", err)
	}
	return s.runGenerate(in)
}

// Generate returns the complete PlatformIO tree of a project, read from dir
// ("" = the project directory; plan 3 passes the job's staging copy).
func (s *Service) Generate(ctx context.Context, projectID string, dir string) (map[string][]byte, error) {
	unlock := s.lockProject(projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return nil, s.internalError("generate", err)
	}
	var root *os.Root
	if dir == "" {
		root, err = s.openProjectRoot(row)
	} else {
		root, err = s.openRoot(row, dir)
	}
	if err != nil {
		return nil, s.internalError("generate", err)
	}
	defer root.Close()
	out, err := s.generateFromRoot(row, root)
	if err != nil {
		return nil, err
	}
	return out.Files, nil
}
