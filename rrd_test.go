package rrd

import (
	"fmt"
	"testing"
	"time"
)

func TestAll(t *testing.T) {
	// Create
	const (
		dbfile    = "/tmp/test.rrd"
		step      = 1
		heartbeat = 2 * step
	)

	c := NewCreator(dbfile, time.Now(), step)
	c.RRA("AVERAGE", 0.5, 1, 100)
	c.RRA("AVERAGE", 0.5, 5, 100)
	c.DS("cnt", "COUNTER", heartbeat, 0, 100)
	c.DS("g", "GAUGE", heartbeat, 0, 60)
	err := c.Create(true)
	if err != nil {
		t.Fatal(err)
	}

	// Update
	u := NewUpdater(dbfile)
	for i := 0; i < 10; i++ {
		time.Sleep(step * time.Second)
		err := u.Update(time.Now(), i, 1.5*float64(i))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Update with cache
	for i := 10; i < 20; i++ {
		time.Sleep(step * time.Second)
		u.Cache(time.Now(), i, 2*float64(i))
	}
	err = u.Update()
	if err != nil {
		t.Fatal(err)
	}

	// Fetch
	end := time.Now()
	start := end.Add(-20 * step * time.Second)
	fmt.Printf("Fetch Params:\n")
	fmt.Printf("Start: %s\n", start)
	fmt.Printf("End: %s\n", end)
	fmt.Printf("Step: %s\n", step*time.Second)
	fetchRes, err := Fetch(dbfile, "AVERAGE", start, end, step*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer fetchRes.FreeValues()
	fmt.Printf("FetchResult:\n")
	fmt.Printf("Start: %s\n", fetchRes.Start)
	fmt.Printf("End: %s\n", fetchRes.End)
	fmt.Printf("Step: %s\n", fetchRes.Step)
	for _, dsName := range fetchRes.DsNames {
		fmt.Printf("\t%s", dsName)
	}
	fmt.Printf("\n")

	row := 0
	for ti := fetchRes.Start.Add(fetchRes.Step); ti.Before(end) || ti.Equal(end); ti = ti.Add(fetchRes.Step) {
		fmt.Printf("%s / %d", ti, ti.Unix())
		for i := 0; i < len(fetchRes.DsNames); i++ {
			v := fetchRes.ValueAt(i, row)
			fmt.Printf("\t%e", v)
		}
		fmt.Printf("\n")
		row++
	}

	// Xport
	end = time.Now()
	start = end.Add(-20 * step * time.Second)
	fmt.Printf("Xport Params:\n")
	fmt.Printf("Start: %s\n", start)
	fmt.Printf("End: %s\n", end)
	fmt.Printf("Step: %s\n", step*time.Second)

	e := NewExporter()
	e.Def("def1", dbfile, "cnt", "AVERAGE")
	e.Def("def2", dbfile, "g", "AVERAGE")
	e.CDef("vdef1", "def1,def2,+")
	e.XportDef("def1", "cnt")
	e.XportDef("def2", "g")
	e.XportDef("vdef1", "sum")

	xportRes, err := e.Xport(start, end, step*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer xportRes.FreeValues()
	fmt.Printf("XportResult:\n")
	fmt.Printf("Start: %s\n", xportRes.Start)
	fmt.Printf("End: %s\n", xportRes.End)
	fmt.Printf("Step: %s\n", xportRes.Step)
	for _, legend := range xportRes.Legends {
		fmt.Printf("\t%s", legend)
	}
	fmt.Printf("\n")

	row = 0
	for ti := xportRes.Start.Add(xportRes.Step); ti.Before(end) || ti.Equal(end); ti = ti.Add(xportRes.Step) {
		fmt.Printf("%s / %d", ti, ti.Unix())
		for i := 0; i < len(xportRes.Legends); i++ {
			v := xportRes.ValueAt(i, row)
			fmt.Printf("\t%e", v)
		}
		fmt.Printf("\n")
		row++
	}
}

func ExampleCreator_DS() {
	c := &Creator{}

	// Add a normal data source, i.e. one of GAUGE, COUNTER, DERIVE and ABSOLUTE:
	c.DS("regular_ds", "DERIVE",
		900, /* heartbeat */
		0,   /* min */
		"U" /* max */)

	// Add a computed
	c.DS("computed_ds", "COMPUTE",
		"regular_ds,8,*" /* RPN expression */)
}

func ExampleCreator_RRA() {
	c := &Creator{}

	// Add a normal consolidation function, i.e. one of MIN, MAX, AVERAGE and LAST:
	c.RRA("AVERAGE",
		0.3, /* xff */
		5,   /* steps */
		1200 /* rows */)

	// Add aberrant behavior detection:
	c.RRA("HWPREDICT",
		1200, /* rows */
		0.4,  /* alpha */
		0.5,  /* beta */
		288 /* seasonal period */)
}
