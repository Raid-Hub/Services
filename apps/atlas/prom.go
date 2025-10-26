package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"raidhub/lib/monitoring"
	"strconv"
	"time"
)

func get404Fraction(intervalMins int) (float64, error) {
	f, err := execWeightedQuery(fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status="3"}[%dm])) / sum(rate(pgcr_crawl_summary_status{}[%dm]))`, intervalMins, intervalMins), intervalMins)
	if err != nil || f == -1 {
		return 0, err
	} else {
		return f, err
	}
}

func getErrorFraction(intervalMins int) (float64, error) {
	f, err := execWeightedQuery(fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status=~"6|7|8|9|10"}[%dm])) / sum(rate(pgcr_crawl_summary_status{}[%dm]))`, intervalMins, intervalMins), intervalMins)
	if err != nil || f == -1 {
		return 0, err
	} else {
		return f, err
	}
}

func getMedianLag(intervalMins int) (float64, error) {
	medianLag, err := execWeightedQuery(`histogram_quantile(0.20, sum(rate(pgcr_crawl_summary_lag_bucket[2m])) by (le))`, intervalMins)
	if err != nil {
		return 0, err
	} else if medianLag == -1 {
		medianLag = 900
	}
	return medianLag, err
}

func get404Rate(intervalMins int) (float64, error) {
	res, err := execQuery(fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status="3"}[%dm])) * %d * 60`, intervalMins, intervalMins), 0)
	if err != nil {
		return 0, err
	} else {
		if len(res.Data.Result) == 0 {
			return 0, nil
		}
		return strconv.ParseFloat(res.Data.Result[0].Values[0][1].(string), 64)
	}
}

func getPgcrsPerSecond(intervalMins int) (float64, error) {
	f, err := execWeightedQuery(fmt.Sprintf(`sum(rate(pgcr_crawl_summary_status{status=~"1|2"}[%dm]))`, intervalMins), intervalMins)
	if err != nil || f == -1 {
		return 0, err
	} else {
		return f, err
	}
}

func execQuery(query string, intervalMins int) (*monitoring.QueryRangeResponse, error) {
	params := url.Values{}
	params.Add("query", query)
	params.Add("start", time.Now().Add(time.Duration(-intervalMins)*time.Minute).Format(time.RFC3339))
	params.Add("end", time.Now().Format(time.RFC3339))
	params.Add("step", "1m")

	client := http.Client{
		Timeout: time.Second * 10,
	}

	url := fmt.Sprintf("http://localhost:9090/api/v1/query_range?%s", params.Encode())

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	var res monitoring.QueryRangeResponse
	err = decoder.Decode(&res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func execWeightedQuery(query string, intervalMins int) (float64, error) {
	res, err := execQuery(query, intervalMins)
	if err != nil {
		return 0, err
	}

	// Creates a weighted average over the interval
	c := 0
	s := 0.0

	if len(res.Data.Result) == 0 {
		return -1, nil
	}

	for idx, y := range res.Data.Result[0].Values {
		val, err := strconv.ParseFloat(y[1].(string), 64)
		if err != nil {
			log.Fatal(err)
		}
		if math.IsNaN(val) {
			continue
		}
		c += (idx + 1)
		s += float64(idx+1) * val
	}

	if c == 0 {
		c = 1
	}

	return s / float64(c), nil
}
