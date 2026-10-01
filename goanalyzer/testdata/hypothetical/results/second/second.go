package second

import "example.com/hypothetical/repo"

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
