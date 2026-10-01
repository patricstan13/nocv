package worker

import "example.com/hypothetical/repo"

func Work() {
	_, _ = repo.Find()
}
