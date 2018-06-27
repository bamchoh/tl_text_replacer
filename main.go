package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"

	_ "github.com/go-sql-driver/mysql"
)

type Node struct {
	ID       uint64
	Name     string
	ParentID uint64
	NodeType uint64
}

type ProjectNode struct {
	ID   uint64
	Name string
}

type Testcase struct {
	ID            uint64
	EXTID         uint64
	Name          string
	ParentID      uint64
	Version       uint64
	Summary       string
	Preconditions string
	Steps         []Step
}

type Step struct {
	ID       uint64
	Step     uint64
	Action   string
	Expected string
}

type TestLinkDB struct {
	*sql.DB
}

func (tldb *TestLinkDB) getProjectNode(name string) (*ProjectNode, error) {
	rows, err := tldb.Query("select id,name from nodes_hierarchy where name = ?", name)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		pn := ProjectNode{}
		rows.Scan(&pn.ID, &pn.Name)
		return &pn, nil
	}
	return nil, errors.New("Project node was not found")
}

func (tldb *TestLinkDB) getTestcaseNodesByID(id uint64) ([]Node, error) {
	rows, err := tldb.Query("select id,name,parent_id,node_type_id from nodes_hierarchy where parent_id = ?", id)
	if err != nil {
		return nil, err
	}

	var nodes []Node
	for rows.Next() {
		n := Node{}
		rows.Scan(&n.ID, &n.Name, &n.ParentID, &n.NodeType)
		switch n.NodeType {
		case 2:
			tmp, err := tldb.getTestcaseNodesByID(n.ID)
			if err != nil {
				return nil, err
			}
			if len(tmp) != 0 {
				nodes = append(nodes, tmp...)
			}
		case 3:
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

func (tldb *TestLinkDB) getTeststepsByID(id uint64) ([]Step, error) {
	rows, err := tldb.Query("select tcsteps.id,step_number,actions,expected_results from tcsteps right join (select id from nodes_hierarchy where parent_id = ? and node_type_id = 9) as nh on nh.id = tcsteps.id", id)
	if err != nil {
		return nil, err
	}

	var nodes []Step
	for rows.Next() {
		n := Step{}
		rows.Scan(&n.ID, &n.Step, &n.Action, &n.Expected)
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (tldb *TestLinkDB) getTestcasesByNodeinfo(nodes []Node) ([]Testcase, error) {
	var ids []string
	for _, node := range nodes {
		ids = append(ids, strconv.FormatUint(node.ID, 10))
	}

	stmt := fmt.Sprintf("select id,tcversions.tc_external_id,version,summary,preconditions from tcversions right join (select tc_external_id,max(version) as MAX_VERSION from tcversions right join (select * from nodes_hierarchy where parent_id in (%v)) as nh on nh.id = tcversions.id group by tc_external_id) tcv1 on tcv1.tc_external_id = tcversions.tc_external_id AND tcv1.MAX_VERSION = tcversions.version", strings.Join(ids, ","))
	rows, err := tldb.Query(stmt)
	if err != nil {
		return nil, err
	}

	var tcs []Testcase
	for rows.Next() {
		tc := Testcase{}
		rows.Scan(&tc.ID, &tc.EXTID, &tc.Version, &tc.Summary, &tc.Preconditions)

		steps, err := tldb.getTeststepsByID(tc.ID)
		if err != nil {
			return nil, errors.Wrap(err, "[getTeststepsByID]")
		}
		tc.Steps = steps

		tcs = append(tcs, tc)
	}
	return tcs, nil
}

func colorablePrint(text, search string, fgcolor color.Attribute) {
	cnt := strings.Count(text, search)
	x := 0
	for i := 0; i < cnt; i++ {
		n := strings.Index(text[x:], search)
		fmt.Print(text[x : x+n])
		color.New(fgcolor).Print(text[x+n : x+n+len(search)])
		x += n + len(search)
	}
	fmt.Print(text[x:])
	if !strings.HasSuffix(text, "\n") {
		fmt.Println()
	}
}

func printReplaceString(tcs []Testcase, search, replace string) {
	for _, tc := range tcs {
		var modSummary string
		if text := strings.Replace(tc.Summary, search, replace, -1); text != tc.Summary {
			modSummary = text
		}

		var modPreconditions string
		if text := strings.Replace(tc.Preconditions, search, replace, -1); text != tc.Preconditions {
			modPreconditions = text
		}

		var modSteps []Step
		for _, step := range tc.Steps {
			var tmp Step
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
				colorablePrint(tc.Summary, search, color.FgRed)
				fmt.Print("   new: ")
				colorablePrint(modSummary, replace, color.FgHiBlue)
			}
			if modPreconditions != "" {
				fmt.Println(" Precond: ")
				fmt.Print("   old: ")
				colorablePrint(tc.Preconditions, search, color.FgRed)
				fmt.Print("   new: ")
				colorablePrint(modPreconditions, replace, color.FgHiBlue)
			}
			for _, step := range modSteps {
				var origStep Step
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
					colorablePrint(origStep.Action, search, color.FgRed)
					fmt.Print("   new: ")
					colorablePrint(step.Action, replace, color.FgHiBlue)
				}
				if step.Expected != origStep.Expected {
					fmt.Println("  Expected: ")
					fmt.Print("   old: ")
					colorablePrint(origStep.Expected, search, color.FgRed)
					fmt.Print("   new: ")
					colorablePrint(step.Expected, replace, color.FgHiBlue)
				}
			}
			fmt.Println()
		}
	}
}

func printSearchString(tcs []Testcase, search string) {
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
				colorablePrint(tc.Summary, search, color.FgRed)
			}
			if foundInPreconditions {
				fmt.Printf(" Precond: ")
				colorablePrint(tc.Preconditions, search, color.FgRed)
			}
			for _, i := range foundSteps {
				fmt.Println(" Step No.:", tc.Steps[i].Step)
				fmt.Printf("  Actions: ")
				colorablePrint(tc.Steps[i].Action, search, color.FgRed)
				fmt.Printf("  Expected: ")
				colorablePrint(tc.Steps[i].Expected, search, color.FgRed)
			}
			fmt.Println()
		}
	}
}

var dbname = flag.String("db", "testlink", "")
var pname = flag.String("project", "", "")
var search = flag.String("search", "", "")
var replace = flag.String("replace", "", "")
var user = flag.String("u", "root", "user name for db")
var pass = flag.String("P", "", "password for db")

func main() {
	flag.Parse()
	if *pname == "" || *search == "" || *replace == "" {
		fmt.Println("Usage")
		fmt.Println("mysql_test -db=<dbname> -project=<projectname> -search=<search string> -replace=<replace string>")
		os.Exit(-1)
	}

	fmt.Println("+==================================")
	fmt.Println("| Target Project Name :", *pname)
	fmt.Println("|      Search  String :", *search)
	fmt.Println("|      Replace String :", *replace)
	fmt.Println("+==================================")
	fmt.Println()
	fmt.Println(" * Are these settings OK? ( YES : Press Enter / NO : Press CTRL+C )")
	bufio.NewScanner(os.Stdin).Scan()

	db, err := sql.Open("mysql", *user+":"+*pass+"@/"+*dbname)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	tldb := TestLinkDB{db}

	pn, err := tldb.getProjectNode(*pname)
	if err != nil {
		panic(err)
	}

	nodes, err := tldb.getTestcaseNodesByID(pn.ID)
	if err != nil {
		panic(err)
	}

	tcs, err := tldb.getTestcasesByNodeinfo(nodes)
	if err != nil {
		panic(err)
	}

	printReplaceString(tcs, *search, *replace)
}
