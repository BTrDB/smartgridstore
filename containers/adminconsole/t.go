package main

import "fmt"

func main() {

	logo := []string{
		"    _____   ______   ______  ",
		"   / ____| /  ____| /  ____| ",
		"  | (___   | |  __ | (____   ",
		"   \\___ \\  | | |_ | \\____ \\  ",
		"   ____) | | |__| |  ____) | ",
		"  |_____/  \\______| |_____/  ",
		"",
		" Smart Grid Store admin console",
		" (c) 2017 Michael Andersen, Sam Kumar",
		" (c) 2017 Regents of the University of California",
		"----------------------------------------------------",
		"",
	}
	for _, l := range logo {
		fmt.Println(l)
	}

}
