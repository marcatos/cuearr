package domain

import (
	"fmt"
	"time"
)

const ImportRequestCooldown = 2 * time.Minute

func BeginImportRequest(job Job, now time.Time) (Job, error) {
	status := job.ImportStatus
	if status == "" {
		status = ImportNone
	}

	switch status {
	case ImportNone, ImportFailed:
	case ImportRequested:
		if now.Before(job.ImportRequestedAt.Add(ImportRequestCooldown)) {
			return job, fmt.Errorf("import request is within %s cooldown", ImportRequestCooldown)
		}
	case ImportImported:
		return job, fmt.Errorf("import already completed")
	default:
		return job, fmt.Errorf("cannot request import from status %q", status)
	}

	job.ImportStatus = ImportRequested
	job.ImportError = ""
	job.ImportRequestedAt = now
	job.ImportFinishedAt = time.Time{}
	return job, nil
}

func CompleteImportOK(job Job, now time.Time) Job {
	job.ImportStatus = ImportImported
	job.ImportError = ""
	job.ImportFinishedAt = now
	return job
}

func CompleteImportFail(job Job, now time.Time, errMsg string) Job {
	job.ImportStatus = ImportFailed
	job.ImportError = errMsg
	job.ImportFinishedAt = now
	return job
}

func SkipImport(job Job, reason string) Job {
	job.ImportStatus = ImportSkipped
	job.ImportError = reason
	return job
}
