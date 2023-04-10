package main

import (
	family "FamilyAccount/utils"
	"fmt"
)

func main() {
	fmt.Println("面向对象完成任务")
	family.NewFamilyAccount().Menu()
}
