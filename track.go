package main

import (
	_ "embed"
	"encoding/csv"
	"html/template"
	"io"
	"log"
	"os"
	"sort"
	"time"

	"github.com/jszwec/csvutil"
)

var (
	//go:embed "template.html"
	mapHTML     string
	mapTemplate = template.Must(template.New("track").Parse(mapHTML))
)

type Row struct {
	Time   time.Time `csv:"time"`
	Lat    float64   `csv:"lat"`
	Lng    float64   `csv:"lng"`
	Height float64   `csv:"height"`
}

// unmarshalTime unmarshal data in CSV to time
func unmarshalTime(data []byte, t *time.Time) error {
	var err error
	*t, err = time.Parse("2006-01-02 15:04:05.000", string(data))
	return err
}

// loadData loads data from CSV file, parses time in loc
func loadData(r io.Reader, loc *time.Location) ([]Row, error) {
	var rows []Row
	dec, err := csvutil.NewDecoder(csv.NewReader(r))
	dec.Register(unmarshalTime)
	if err != nil {
		return nil, err
	}

	for {
		var row Row
		err := dec.Decode(&row)

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		row.Time = row.Time.In(loc)
		rows = append(rows, row)
	}

	return rows, nil
}

// meanRow returns rows that has mean values for rows
func meanRow(t time.Time, rows []Row) Row {
	lat, lng, height := 0.0, 0.0, 0.0
	for _, row := range rows {
		lat += row.Lat
		lng += row.Lng
		height += row.Height
	}

	count := float64(len(rows))
	return Row{
		Time:   t,
		Lat:    lat / count,
		Lng:    lng / count,
		Height: height / count,
	}
}

// resample re-samples rows to freq, using mean to calculate values
func resample(rows []Row, freq time.Duration) []Row {
	buckets := make(map[time.Time][]Row)
	for _, row := range rows {
		t := row.Time.Truncate(freq)
		buckets[t] = append(buckets[t], row)
	}

	out := make([]Row, 0, len(buckets))
	for t, rows := range buckets {
		out = append(out, meanRow(t, rows))
	}

	sort.Slice(out, func(i, j int) bool { return rows[i].Time.Before(rows[j].Time) })
	return out
}

func main() {
	file, err := os.Open("track.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	loc, err := time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		log.Fatal(err)
	}

	rows, err := loadData(file, loc)
	if err != nil {
		log.Fatal(err)
	}

	rows = resample(rows, time.Minute)

	// Find token in https://account.mapbox.com/access-tokens/
	accessToken := os.Getenv("MAPBOX_TOKEN")
	if accessToken == "" {
		log.Fatal("error: no access token, did you set MAPBOX_TOKEN?")
	}

	// Template data
	data := map[string]interface{}{
		"start":        rows[len(rows)/2],
		"rows":         rows,
		"access_token": accessToken,
	}
	if err := mapTemplate.Execute(os.Stdout, data); err != nil {
		log.Fatal(err)
	}
}
