package count

import "example.com/hypothetical/repo"

func Added() {
	u := repo.Single()
	_ = u
}

func DiscardAdded() {
	repo.Single()
}

func Removed() {
	u, err := repo.Pair()
	_, _ = u, err
}
