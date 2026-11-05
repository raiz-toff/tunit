package menu

import (
	"fmt"
	"os"
	"strings"

	"github.com/tunit/tunit/internal/report"
)

type MainChoice int

const (
	ChoiceFullQC MainChoice = iota + 1
	ChoiceStress
	ChoiceNetwork
	ChoiceLoadTest
	ChoiceLastReport
)

type RepairType int

const (
	RepairNone RepairType = iota + 1
	RepairScreen
	RepairMotherboard
	RepairBattery
	RepairStorage
	RepairKeyboard
	RepairOther
)

func (r RepairType) String() string {
	switch r {
	case RepairNone:
		return "No repair"
	case RepairScreen:
		return "Screen / Display"
	case RepairMotherboard:
		return "Motherboard"
	case RepairBattery:
		return "Battery"
	case RepairStorage:
		return "Storage (SSD/HDD)"
	case RepairKeyboard:
		return "Keyboard / Top case"
	default:
		return "Other"
	}
}

func AskMain() MainChoice {
	report.PrintBanner()
	fmt.Print("  What would you like to do?\n\n")
	fmt.Println("    1 › Full Computer Check  ← recommended for most cases")
	fmt.Println("    2 › Stress Test Only     ← CPU & thermals")
	fmt.Println("    3 › Network Test")
	fmt.Println("    4 › Speed & Load Test    ← for developers / IT")
	fmt.Println("    5 › View Last Report")
	fmt.Println()

	return MainChoice(askNumber("  Enter a number [1-5]: ", 1, 5))
}

func AskRepairType() RepairType {
	fmt.Print("\n  Was any repair done on this unit?\n\n")
	fmt.Println("    1 › No repair — fresh intake")
	fmt.Println("    2 › Screen / Display replaced")
	fmt.Println("    3 › Motherboard replaced")
	fmt.Println("    4 › Battery replaced")
	fmt.Println("    5 › Storage (SSD / HDD) replaced")
	fmt.Println("    6 › Keyboard or top case replaced")
	fmt.Println("    7 › Other / Not sure")
	fmt.Println()

	return RepairType(askNumber("  Enter a number [1-7]: ", 1, 7))
}

func AskStressTier() int {
	fmt.Print("\n  Stress test type:\n\n")
	fmt.Println("    1 › Quick check (20s)  — CPU load, fan spin, thermal baseline")
	fmt.Println("    2 › Standard (60s)     — Recommended for most units")
	fmt.Println("    3 › Heavy (120s)       — Post-motherboard or cooling repair")
	fmt.Println("    4 › GPU stress         — Downloads FurMark, requires internet")
	fmt.Println()

	return askNumber("  Enter a number [1-4]: ", 1, 4)
}

func AskLoadTestParams() (url, method string, rps int, duration string) {
	fmt.Print("\n  HTTP Load Test\n\n")
	fmt.Print("  Target URL: ")
	fmt.Fscan(os.Stdin, &url)

	fmt.Print("  Method [GET]: ")
	fmt.Fscan(os.Stdin, &method)
	if strings.TrimSpace(method) == "" {
		method = "GET"
	}

	fmt.Print("  Requests per second [50]: ")
	var rpsStr string
	fmt.Fscan(os.Stdin, &rpsStr)
	rps = 50
	fmt.Sscanf(rpsStr, "%d", &rps)

	fmt.Print("  Duration [30s]: ")
	fmt.Fscan(os.Stdin, &duration)
	if strings.TrimSpace(duration) == "" {
		duration = "30s"
	}

	return
}

func askNumber(prompt string, min, max int) int {
	for {
		fmt.Print(prompt)
		var n int
		_, err := fmt.Fscan(os.Stdin, &n)
		if err == nil && n >= min && n <= max {
			return n
		}
		fmt.Printf("  Please enter a number between %d and %d: ", min, max)
	}
}
