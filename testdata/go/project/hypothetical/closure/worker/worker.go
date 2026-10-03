package worker

import "example.com/shop/hypothetical/closure/repo"

func Work() int {
	return repo.Find()
}
