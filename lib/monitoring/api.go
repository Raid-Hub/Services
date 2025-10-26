package monitoring

type QueryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Values [][]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}
