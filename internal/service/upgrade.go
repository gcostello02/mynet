package service

import (
	"fmt"
	"os"
)

// UpgradeStages copies candidate to staged path after metadata verification.
type UpgradePlan struct {
	LivePath     string
	CandidatePath string
	BackupPath   string
	StagedPath   string
}

func PlanUpgrade(livePath, candidatePath, backupPath string) (UpgradePlan, error) {
	if livePath == "" || candidatePath == "" {
		return UpgradePlan{}, fmt.Errorf("live and candidate paths required")
	}
	if backupPath == "" {
		backupPath = livePath + ".rollback"
	}
	return UpgradePlan{
		LivePath:      livePath,
		CandidatePath: candidatePath,
		BackupPath:    backupPath,
		StagedPath:    livePath + ".next",
	}, nil
}

// StageCandidate copies the candidate beside live for atomic swap (no running process stop).
func StageCandidate(plan UpgradePlan) error {
	if err := copyExecutable(plan.CandidatePath, plan.StagedPath); err != nil {
		return err
	}
	return nil
}

// CommitSwap atomically replaces live with staged after the daemon is stopped.
func CommitSwap(plan UpgradePlan) error {
	if _, err := os.Stat(plan.StagedPath); err != nil {
		return fmt.Errorf("staged binary missing: %w", err)
	}
	if _, err := os.Stat(plan.LivePath); err == nil {
		if err := copyExecutable(plan.LivePath, plan.BackupPath); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	if err := os.Rename(plan.StagedPath, plan.LivePath); err != nil {
		return err
	}
	return nil
}

// Rollback restores backup over live.
func Rollback(plan UpgradePlan) error {
	if _, err := os.Stat(plan.BackupPath); err != nil {
		return fmt.Errorf("rollback binary missing: %w", err)
	}
	return copyExecutable(plan.BackupPath, plan.LivePath)
}

func copyExecutable(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
