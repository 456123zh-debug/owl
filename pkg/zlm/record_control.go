package zlm

const (
	startRecordPath = "/index/api/startRecord"
	stopRecordPath  = "/index/api/stopRecord"
)

type RecordControlRequest struct {
	Vhost          string `json:"vhost"`
	App            string `json:"app"`
	Stream         string `json:"stream"`
	CustomizedPath string `json:"customized_path,omitempty"`
}

type RecordControlResponse struct {
	FixedHeader
	Result bool `json:"result"`
}

func (e *Engine) StartHLSRecord(req RecordControlRequest) error {
	data := map[string]any{"type": 1, "vhost": req.Vhost, "app": req.App, "stream": req.Stream}
	if req.CustomizedPath != "" {
		data["customized_path"] = req.CustomizedPath
	}
	var resp RecordControlResponse
	if err := e.post(startRecordPath, data, &resp); err != nil {
		return err
	}
	return e.ErrHandle(resp.Code, resp.Msg)
}

func (e *Engine) StopHLSRecord(req RecordControlRequest) error {
	var resp RecordControlResponse
	if err := e.post(stopRecordPath, map[string]any{"type": 1, "vhost": req.Vhost, "app": req.App, "stream": req.Stream}, &resp); err != nil {
		return err
	}
	return e.ErrHandle(resp.Code, resp.Msg)
}
