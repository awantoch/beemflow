package main

func main() {
	if err := NewRootCmd().Execute(); err != nil {
		exit(1)
	}
}
