package worker

import "example.com/closure/repo"

func Work() int {
	return repo.Find()
}
