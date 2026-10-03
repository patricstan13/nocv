package second

import "example.com/shop/hypothetical/results/repo"

func consumeError(error) {}

func Ignored() {
	u, _ := repo.Second()
	_ = u
}

func Inferred() {
	u, status := repo.Second()
	_, _ = u, status
}

func Required() {
	_, err := repo.Second()
	consumeError(err)
}
