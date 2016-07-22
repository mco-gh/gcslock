package cloudmutex

import (
	"fmt"
)

func Lock() {
	fmt.Println("locked")
}

func Unlock() {
	fmt.Println("unlocked")
}
