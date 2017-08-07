package main

import (
	"database/sql"
	"fmt"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"time"

	ini "gopkg.in/ini.v1"

	ui "github.com/gizak/termui"
	"github.com/murdinc/awsm/aws"

	_ "github.com/go-sql-driver/mysql"
)

// Config that stores the db, user, and password, loaded from ~/.thirteen
type ThirteenConfig struct {
	DB             string
	Port           string
	User           string
	Password       string
	ItemCountQuery string
}

// Slice of DB Stats
type DbStats []DbStat

// A single DB Stat
type DbStat struct {
	DB                  *sql.DB
	Name                string
	Class               string
	Sequence            string
	Locale              string
	QueryCount          int
	ItemCount           int
	SlaveIORunning      bool
	SlaveSQLRunning     bool
	SecondsBehindMaster int
	MasterLogFile       string
	MasterPosition      int
	IPAddress           string
	GatherTime          time.Time
}

// Loads in the DB config from ~/.thirteen
func ReadConfig() (*ThirteenConfig, error) {
	config := new(ThirteenConfig)
	currentUser, _ := user.Current()
	configLocation := currentUser.HomeDir + "/.thirteen"

	thirteen, err := ini.Load(configLocation)
	if err != nil {
		return config, err
	}

	config.DB = thirteen.Section("").Key("DB").String()
	config.Port = thirteen.Section("").Key("PORT").String()
	config.User = thirteen.Section("").Key("USER").String()
	config.Password = thirteen.Section("").Key("PASSWORD").String()
	config.ItemCountQuery = thirteen.Section("").Key("ITEMCOUNTQUERY").String()

	return config, nil
}

func (d DbStats) Len() int {
	return len(d)
}

func (d DbStats) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d DbStats) Less(i, j int) bool {
	if d[i].Class == "mysql-master" && d[j].Class != "mysql-master" {
		return true
	}
	if d[i].Class == "mysql-backup" {
		return false
	}
	if d[j].Class == "mysql-backup" {
		return true
	}
	return d[i].Sequence < d[j].Sequence
}

func main() {

	fmt.Println("Loading thirteen config...\n")
	config, err := ReadConfig()
	if err != nil {
		fmt.Println("Error gathering thirteen config!\n")
		fmt.Println(err.Error())

		return
	}

	fmt.Println("Looking for MySQL Instances...\n")

	instances, errs := aws.GetInstances("mysql-master|mysql-read|mysql-backup", true)

	// Bail on errors
	if len(errs) > 0 {
		fmt.Println("Error gathering instance list!\n")
		return
	}

	fmt.Printf("Found %d Instances, attempting to open MySQL connection(s)...\n", len(*instances))

	stats := new(DbStats)

	for i, inst := range *instances {

		fmt.Printf("[%d] Opening connection to: %s @ %s ...\n", i+1, inst.Name, inst.PrivateIP)

		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp([%s]:%s)/%s", config.User, config.Password, inst.PrivateIP, config.Port, config.DB))
		if err != nil {
			print(err.Error())
			return
		}

		// Open doesn't open a connection. Validate DSN data:
		err = db.Ping()
		if err != nil {
			fmt.Printf("	> Error! Unable to open connection to: %s @ %s, skipping ...\n", inst.Name, inst.PrivateIP)
			continue
		}

		*stats = append(*stats, DbStat{
			DB:        db,
			Name:      inst.Name,
			Class:     inst.Class,
			Sequence:  strings.TrimPrefix(inst.Name, inst.Class),
			Locale:    inst.Region,
			IPAddress: inst.PrivateIP,
		})
	}

	fmt.Println("Connections established! Gathering data...\n")

	// Sort the servers
	sort.Sort(*stats)

	for i, _ := range *stats {
		go (*stats)[i].GatherData(config.ItemCountQuery)
	}

	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

	// Query Count
	qc := ui.NewBarChart()
	qc.BorderLabel = "Query Count"
	qc.BarGap = 4
	qc.Width = 200
	qc.Height = 8
	qc.X = 0
	qc.Y = 0
	qc.BarColor = ui.ColorRed
	qc.NumColor = ui.ColorWhite
	qc.BarWidth = 13

	// Item Count
	ic := ui.NewBarChart()
	ic.BorderLabel = "Item Count"
	ic.BarGap = 4
	ic.Width = 200
	ic.Height = 4
	ic.BarColor = ui.ColorYellow
	ic.NumColor = ui.ColorWhite
	ic.BarWidth = 13

	// Master Position
	mp := ui.NewBarChart()
	mp.BorderLabel = "Master Position"
	mp.BarGap = 4
	mp.Width = 200
	mp.Height = 4
	mp.BarColor = ui.ColorGreen
	mp.NumColor = ui.ColorWhite
	mp.BarWidth = 13

	// Slave IO Running
	sior := ui.NewBarChart()
	sior.BorderLabel = "Slave IO Running"
	sior.BarGap = 4
	sior.Width = 200
	sior.Height = 4
	sior.BarColor = ui.ColorCyan
	sior.NumColor = ui.ColorCyan
	sior.BarWidth = 13

	// Slave SQL Running
	ssr := ui.NewBarChart()
	ssr.BorderLabel = "Slave SQL Running"
	ssr.BarGap = 4
	ssr.Width = 200
	ssr.Height = 4
	ssr.BarColor = ui.ColorBlue
	ssr.NumColor = ui.ColorBlue
	ssr.BarWidth = 13

	// Seconds Behind Master
	sbm := ui.NewBarChart()
	sbm.BorderLabel = "Seconds Behind Master"
	sbm.BarGap = 4
	sbm.Width = 200
	sbm.Height = 4
	sbm.BarColor = ui.ColorRed
	sbm.NumColor = ui.ColorWhite
	sbm.BarWidth = 13

	// Totals
	spdata := make([]float64, 200)

	tot0 := ui.NewLineChart()
	tot0.Mode = "dot"
	tot0.Height = 9
	tot0.Data = spdata
	tot0.LineColor = ui.ColorCyan
	tot0.SetY(50)

	tot1 := ui.NewLineChart()
	tot1.Mode = "dot"
	tot1.Height = 9
	tot1.Data = spdata
	tot1.LineColor = ui.ColorCyan

	tot2 := ui.NewLineChart()
	tot2.Mode = "dot"
	tot2.Height = 9
	tot2.Data = spdata
	tot2.LineColor = ui.ColorCyan

	tot3 := ui.NewLineChart()
	tot3.Mode = "dot"
	tot3.Height = 9
	tot3.Data = spdata
	tot3.LineColor = ui.ColorCyan

	// Layout
	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, qc),
		),
		ui.NewRow(
			ui.NewCol(12, 0, ic),
		),
		ui.NewRow(
			ui.NewCol(12, 0, mp),
		),
		ui.NewRow(
			ui.NewCol(12, 0, sior),
		),
		ui.NewRow(
			ui.NewCol(12, 0, ssr),
		),
		ui.NewRow(
			ui.NewCol(12, 0, sbm),
		),
		ui.NewRow(
			ui.NewCol(12, 0, tot0),
		),
		ui.NewRow(
			ui.NewCol(12, 0, tot1),
		),
		ui.NewRow(
			ui.NewCol(12, 0, tot2),
		),
		ui.NewRow(
			ui.NewCol(12, 0, tot3),
		),
	)

	var tot0Data, tot1Data, tot2Data, tot3Data []float64

	draw := func() {

		labels := make([]string, len(*stats))
		queryCounts := make([]int, len(*stats))
		itemCounts := make([]int, len(*stats))
		mps := make([]int, len(*stats))
		siors := make([]int, len(*stats))
		ssrs := make([]int, len(*stats))
		sbms := make([]int, len(*stats))

		var tott1, tott2, tott3, tot1count, tot2count, tot3count int

		for i, stat := range *stats {
			if stat.SlaveIORunning {
				siors[i] = 1
			}
			if stat.SlaveSQLRunning {
				ssrs[i] = 1
			}

			labels[i] = stat.Name
			queryCounts[i] = stat.QueryCount
			itemCounts[i] = stat.ItemCount
			mps[i] = stat.MasterPosition
			sbms[i] = stat.SecondsBehindMaster

			switch stat.Locale {
			case "us-west-2":
				tott1 += stat.QueryCount
				tot1count++
			case "us-east-1":
				tott2 += stat.QueryCount
				tot2count++
			case "eu-west-1":
				tott3 += stat.QueryCount
				tot3count++
			}
		}

		tot0Queries := float64(tott1 + tott2 + tott3)

		tot1Queries := float64(tott1)
		tot2Queries := float64(tott2)
		tot3Queries := float64(tott3)

		tot0Data = append([]float64{tot0Queries}, tot0Data...)
		if len(tot0Data) > 200 {
			tot0Data = tot0Data[:200]
		}

		tot1Data = append([]float64{tot1Queries}, tot1Data...)
		if len(tot1Data) > 200 {
			tot1Data = tot1Data[:200]
		}

		tot2Data = append([]float64{tot2Queries}, tot2Data...)
		if len(tot2Data) > 200 {
			tot2Data = tot2Data[:200]
		}

		tot3Data = append([]float64{tot3Queries}, tot3Data...)
		if len(tot3Data) > 200 {
			tot3Data = tot3Data[:200]
		}

		qc.DataLabels = labels
		qc.Data = queryCounts

		ic.DataLabels = labels
		ic.Data = itemCounts

		mp.DataLabels = labels
		mp.Data = mps

		sior.DataLabels = labels
		sior.Data = siors

		ssr.DataLabels = labels
		ssr.Data = ssrs

		sbm.DataLabels = labels
		sbm.Data = sbms

		tot0.Data = tot0Data
		tot0.BorderLabel = fmt.Sprintf("Overall [%d]", tott1+tott2+tott3)

		tot1.Data = tot1Data
		tot1.BorderLabel = fmt.Sprintf("us-west-2 [%d]", tott1)

		tot2.Data = tot2Data
		tot2.BorderLabel = fmt.Sprintf("us-east-1 [%d]", tott2)

		tot3.Data = tot3Data
		tot3.BorderLabel = fmt.Sprintf("eu-west-1 [%d]", tott3)

		// calculate layout
		ui.Body.Align()
		ui.Render(ui.Body)
	}

	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})

	ui.Handle("/timer/1s", func(e ui.Event) {
		draw()
	})

	ui.Loop()

}

func (d *DbStat) GatherData(itemCountQuery string) {

	defer d.DB.Close()

	for {

		// Query Count
		queryCountRows, err := d.DB.Query("SELECT COUNT(*) FROM information_schema.PROCESSLIST")
		defer queryCountRows.Close()
		if err != nil {
			queryCountRows.Close()
			// Sleep 1 second...
			time.Sleep(time.Second * 1)
			continue
		}

		var queryCount int

		if queryCountRows.Next() {
			err = queryCountRows.Scan(&queryCount)
			if err != nil {
				print(err.Error())
				return
			}

			d.QueryCount = queryCount
			d.GatherTime = time.Now()
		}
		queryCountRows.Close()

		// Item Count
		itemCountRows, err := d.DB.Query(itemCountQuery)
		defer itemCountRows.Close()
		if err != nil {
			print(err.Error())
			return
		}

		var itemCount int

		if itemCountRows.Next() {
			err = itemCountRows.Scan(&itemCount)
			if err != nil {
				print(err.Error())
				return
			}

			d.ItemCount = itemCount
		}
		itemCountRows.Close()

		// Slave Status
		slaveStatusRows, err := d.DB.Query("SHOW SLAVE STATUS")
		defer slaveStatusRows.Close()
		if err != nil {
			print(err.Error())
			return
		}

		cols, _ := slaveStatusRows.Columns()

		// Make a slice for the values
		values := make([]sql.RawBytes, len(cols))

		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		// Fetch slaveStatusRows
		for slaveStatusRows.Next() {

			err = slaveStatusRows.Scan(scanArgs...)
			if err != nil {
				print(err.Error())
				return
			}

			// Now do something with the data.
			var value string
			for i, col := range values {
				// Here we can check if the value is nil (NULL value)
				if col == nil {
					value = "NULL"
				} else {
					value = string(col)
				}

				switch cols[i] {
				case "Slave_IO_Running":
					if string(value) == "Yes" {
						d.SlaveIORunning = true
					} else {
						d.SlaveIORunning = false
					}
				case "Slave_SQL_Running":
					if string(value) == "Yes" {
						d.SlaveSQLRunning = true
					} else {
						d.SlaveSQLRunning = false
					}
				case "Seconds_Behind_Master":
					d.SecondsBehindMaster, _ = strconv.Atoi(value)

				case "Master_Log_File":
					d.MasterLogFile = string(value)

				case "Read_Master_Log_Pos":
					d.MasterPosition, _ = strconv.Atoi(value)
				}
			}
		}
		if err = slaveStatusRows.Err(); err != nil {
			print(err.Error())
			return
		}

		slaveStatusRows.Close()

		// Sleep 1 second...
		time.Sleep(time.Second * 1)
	}
}
