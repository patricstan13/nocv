package compatibleforward

import "example.com/shop/hypothetical/results/repo"

func Get() (any, error) {
	return repo.Compatible()
}
