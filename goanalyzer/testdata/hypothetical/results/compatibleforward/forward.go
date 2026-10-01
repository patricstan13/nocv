package compatibleforward

import "example.com/hypothetical/repo"

func Get() (any, error) {
	return repo.Compatible()
}
