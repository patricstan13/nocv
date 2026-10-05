package count

import "example.com/shop/hypothetical/results/repo"

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
