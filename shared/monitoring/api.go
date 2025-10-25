package monitoring

type QueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Values [][]interface{} `json:"values"`
		} `json:"result"`
	} `json:"data"`
}
