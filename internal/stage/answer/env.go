package answer

import (
	"errors"
	"path/filepath"
)

type SealedExecutionEnv struct {
	CWD          string `json:"cwd"`
	HomeDir      string `json:"home_dir"`
	CodexHomeDir string `json:"codex_home_dir"`
}

func sealedExecutionEnvForRoot(isolatedRoot string) SealedExecutionEnv {
	workspaceDir := filepath.Join(isolatedRoot, "workspace")
	homeDir := filepath.Join(isolatedRoot, "home")
	return SealedExecutionEnv{
		CWD:          workspaceDir,
		HomeDir:      homeDir,
		CodexHomeDir: filepath.Join(homeDir, ".codex"),
	}
}

func (env SealedExecutionEnv) ValidateForRoot(isolatedRoot string) error {
	expected := sealedExecutionEnvForRoot(isolatedRoot)
	switch {
	case env.CWD != expected.CWD:
		return errors.New("sealed execution environment mismatch")
	case env.HomeDir != expected.HomeDir:
		return errors.New("sealed execution environment mismatch")
	case env.CodexHomeDir != expected.CodexHomeDir:
		return errors.New("sealed execution environment mismatch")
	default:
		return nil
	}
}

func (env SealedExecutionEnv) LaunchContext(extraArgs []string) AppServerLaunchContext {
	return AppServerLaunchContext{
		ExtraArgs:               append([]string(nil), extraArgs...),
		CurrentWorkingDirectory: env.CWD,
		HomeDir:                 env.HomeDir,
		CodexHomeDir:            env.CodexHomeDir,
	}
}
