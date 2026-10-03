package worker

import "example.com/shop/hypothetical/results/repo"

func Work() {
	_, _ = repo.Find()
}
