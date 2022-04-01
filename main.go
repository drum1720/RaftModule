package main

import "fmt"

func main() {
	rm, err := NewRaftModule(6890, 5)
	if err==nil {
		rm.Start()
	}else {
		fmt.Println(err)
	}
}
