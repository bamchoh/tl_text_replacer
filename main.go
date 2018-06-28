package tldb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

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

type ReplaceCandidate struct {
	ID   uint64
	Type string
	Keys []string
	Text map[string]string
}

type TestLinkDB struct {
	*sql.DB
}

func Open(driverName, dataSourceName string) (*TestLinkDB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return &TestLinkDB{db}, nil
}

func (tldb *TestLinkDB) GetProjectNode(name string) (*ProjectNode, error) {
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

func (tldb *TestLinkDB) GetTestcaseNodesByID(id uint64) ([]Node, error) {
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
			tmp, err := tldb.GetTestcaseNodesByID(n.ID)
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

func (tldb *TestLinkDB) GetTeststepsByID(id uint64) ([]Step, error) {
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

func (tldb *TestLinkDB) GetTestcasesByNodeinfo(nodes []Node) ([]Testcase, error) {
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

		steps, err := tldb.GetTeststepsByID(tc.ID)
		if err != nil {
			return nil, errors.Wrap(err, "[getTeststepsByID]")
		}
		tc.Steps = steps

		tcs = append(tcs, tc)
	}
	return tcs, nil
}

func (tldb *TestLinkDB) ReplaceByCandidates(rcs []ReplaceCandidate) error {
	for _, rc := range rcs {
		switch rc.Type {
		case "testcase":
			var setStmt []string
			var setVals []interface{}
			for _, k := range rc.Keys {
				setStmt = append(setStmt, fmt.Sprintf("%v = ?", k))
				setVals = append(setVals, rc.Text[k])
			}
			setVals = append(setVals, rc.ID)
			_, err := tldb.Query("update tcversions set "+strings.Join(setStmt, ",")+" where id = ?", setVals...)
			if err != nil {
				return err
			}
		case "step":
			var setStmt []string
			var setVals []interface{}
			for _, k := range rc.Keys {
				setStmt = append(setStmt, fmt.Sprintf("%v = ?", k))
				setVals = append(setVals, rc.Text[k])
			}
			setVals = append(setVals, rc.ID)
			_, err := tldb.Query("update tcsteps set "+strings.Join(setStmt, ",")+" where id = ?", setVals...)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func GenerateReplaceCandidates(tcs []Testcase, search, replace string) []ReplaceCandidate {
	var rcs []ReplaceCandidate
	for _, tc := range tcs {
		rc := ReplaceCandidate{}
		rc.Text = make(map[string]string)
		if text := strings.Replace(tc.Summary, search, replace, -1); text != tc.Summary {
			rc.ID = tc.ID
			rc.Type = "testcase"
			rc.Keys = append(rc.Keys, "summary")
			rc.Text["summary"] = text
		}
		if text := strings.Replace(tc.Preconditions, search, replace, -1); text != tc.Preconditions {
			rc.ID = tc.ID
			rc.Type = "testcase"
			rc.Keys = append(rc.Keys, "preconditions")
			rc.Text["precondition"] = text
		}
		if rc.ID != 0 {
			rcs = append(rcs, rc)
		}
		for _, step := range tc.Steps {
			rc := ReplaceCandidate{}
			rc.Text = make(map[string]string)
			if text := strings.Replace(step.Action, search, replace, -1); text != step.Action {
				rc.ID = step.ID
				rc.Type = "step"
				rc.Keys = append(rc.Keys, "actions")
				rc.Text["action"] = text
			}
			if text := strings.Replace(step.Expected, search, replace, -1); text != step.Expected {
				rc.ID = step.ID
				rc.Type = "step"
				rc.Keys = append(rc.Keys, "expected_results")
				rc.Text["expected"] = text
			}
			if rc.ID != 0 {
				rcs = append(rcs, rc)
			}
		}
	}
	return rcs
}
