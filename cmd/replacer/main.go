package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bamchoh/tl_text_replacer"
	"github.com/fatih/color"
)

var dbname = flag.String("db", "testlink", "")
var pname = flag.String("project", "", "")
var search = flag.String("search", "", "")
var replace = flag.String("replace", "", "")
var user = flag.String("u", "root", "user name for db")
var pass = flag.String("P", "", "password for db")

func colorablePrint(text, search string, fgcolor, bgcolor color.Attribute) {
	cnt := strings.Count(text, search)
	x := 0
	for i := 0; i < cnt; i++ {
		n := strings.Index(text[x:], search)
		fmt.Print(text[x : x+n])
		color.New(fgcolor).Add(bgcolor).Print(text[x+n : x+n+len(search)])
		x += n + len(search)
	}
	fmt.Print(text[x:])
	if !strings.HasSuffix(text, "\n") {
		fmt.Println()
	}
}

func printReplaceCandidates(rcs []tldb.ReplaceCandidate) {
	for _, rc := range rcs {
		color.Green("=== ID : %v ===\n", rc.ID)
		fmt.Println("Type:", rc.Type)
		for _, k := range rc.Keys {
			fmt.Print(strings.Title(k), ":")
			colorablePrint(rc.Text[k], *replace, color.FgBlack, color.BgYellow)
		}
	}
}

func printReplaceString(tcs []tldb.Testcase, search, replace string) {
	for _, tc := range tcs {
		var modSummary string
		if text := strings.Replace(tc.Summary, search, replace, -1); text != tc.Summary {
			modSummary = text
		}

		var modPreconditions string
		if text := strings.Replace(tc.Preconditions, search, replace, -1); text != tc.Preconditions {
			modPreconditions = text
		}

		var modSteps []tldb.Step
		for _, step := range tc.Steps {
			var tmp tldb.Step
			tmp.Action = strings.Replace(step.Action, search, replace, -1)
			tmp.Expected = strings.Replace(step.Expected, search, replace, -1)

			if tmp.Action != step.Action || tmp.Expected != step.Expected {
				tmp.ID = step.ID
				tmp.Step = step.Step
				modSteps = append(modSteps, tmp)
			}
		}

		if modSummary != "" || modPreconditions != "" || len(modSteps) != 0 {
			color.Green("=== Test Case ID : %v (Internal : %v) ===\n", tc.EXTID, tc.ID)
			if modSummary != "" {
				fmt.Println(" Summary: ")
				fmt.Print("   old: ")
				colorablePrint(tc.Summary, search, color.FgRed, color.BgBlack)
				fmt.Print("   new: ")
				colorablePrint(modSummary, replace, color.FgHiBlue, color.BgBlack)
			}
			if modPreconditions != "" {
				fmt.Println(" Precond: ")
				fmt.Print("   old: ")
				colorablePrint(tc.Preconditions, search, color.FgRed, color.BgBlack)
				fmt.Print("   new: ")
				colorablePrint(modPreconditions, replace, color.FgHiBlue, color.BgBlack)
			}
			for _, step := range modSteps {
				var origStep tldb.Step
				for i := 0; i < len(tc.Steps); i++ {
					if tc.Steps[i].Step == step.Step {
						origStep = tc.Steps[i]
						break
					}
				}
				fmt.Println(" Step No.:", origStep.Step)
				if step.Action != origStep.Action {
					fmt.Println("  Actions: ")
					fmt.Print("   old: ")
					colorablePrint(origStep.Action, search, color.FgRed, color.BgBlack)
					fmt.Print("   new: ")
					colorablePrint(step.Action, replace, color.FgHiBlue, color.BgBlack)
				}
				if step.Expected != origStep.Expected {
					fmt.Println("  Expected: ")
					fmt.Print("   old: ")
					colorablePrint(origStep.Expected, search, color.FgRed, color.BgBlack)
					fmt.Print("   new: ")
					colorablePrint(step.Expected, replace, color.FgHiBlue, color.BgBlack)
				}
			}
			fmt.Println()
		}
	}
}

func printSearchString(tcs []tldb.Testcase, search string) {
	for _, tc := range tcs {
		foundInSummary := false
		foundInPreconditions := false
		var foundSteps []int
		if strings.Contains(tc.Summary, search) {
			foundInSummary = true
		}
		if strings.Contains(tc.Preconditions, search) {
			foundInPreconditions = true
		}
		for i, step := range tc.Steps {
			if strings.Contains(step.Action, search) ||
				strings.Contains(step.Expected, search) {
				foundSteps = append(foundSteps, i)
			}
		}

		if foundInSummary || foundInPreconditions || len(foundSteps) != 0 {
			color.Green("=== Test Case ID : %v (External ID: %v) ===\n", tc.ID, tc.EXTID)
			if foundInSummary {
				fmt.Print(" Summary: ")
				colorablePrint(tc.Summary, search, color.FgRed, color.BgBlack)
			}
			if foundInPreconditions {
				fmt.Printf(" Precond: ")
				colorablePrint(tc.Preconditions, search, color.FgRed, color.BgBlack)
			}
			for _, i := range foundSteps {
				fmt.Println(" Step No.:", tc.Steps[i].Step)
				fmt.Printf("  Actions: ")
				colorablePrint(tc.Steps[i].Action, search, color.FgRed, color.BgBlack)
				fmt.Printf("  Expected: ")
				colorablePrint(tc.Steps[i].Expected, search, color.FgRed, color.BgBlack)
			}
			fmt.Println()
		}
	}
}
func run() int {
	flag.Parse()
	if *pname == "" || *search == "" || *replace == "" {
		fmt.Println("Usage")
		fmt.Println(os.Args[0], "-db=<dbname> -project=<projectname> -search=<search string> -replace=<replace string>")
		return -1
	}

	fmt.Println("+==================================")
	fmt.Println("| Target Project Name :", *pname)
	fmt.Println("|      Search  String :", *search)
	fmt.Println("|      Replace String :", *replace)
	fmt.Println("+==================================")
	fmt.Println()
	fmt.Println(" * Are these settings OK? ( YES : Press Enter / NO : Press CTRL+C )")
	bufio.NewScanner(os.Stdin).Scan()

	db, err := tldb.Open("mysql", *user+":"+*pass+"@/"+*dbname)
	if err != nil {
		fmt.Println(err)
		return -2
	}
	defer db.Close()

	pn, err := db.GetProjectNode(*pname)
	if err != nil {
		fmt.Println(err)
		return -3
	}

	nodes, err := db.GetTestcaseNodesByID(pn.ID)
	if err != nil {
		fmt.Println(err)
		return -4
	}

	tcs, err := db.GetTestcasesByNodeinfo(nodes)
	if err != nil {
		fmt.Println(err)
		return -5
	}

	rcs := tldb.GenerateReplaceCandidates(tcs, *search, *replace)

	printReplaceCandidates(rcs)

	err = db.ReplaceByCandidates(rcs)
	if err != nil {
		fmt.Println(err)
		return -6
	}

	return 0
}

func main() {
	os.Exit(run())
}
