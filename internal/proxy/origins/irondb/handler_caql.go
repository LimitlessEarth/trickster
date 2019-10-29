/**
* Copyright 2018 Comcast Cable Communications Management, LLC
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
* http://www.apache.org/licenses/LICENSE-2.0
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package irondb

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Comcast/trickster/internal/proxy/engines"
	"github.com/Comcast/trickster/internal/proxy/errors"
	"github.com/Comcast/trickster/internal/proxy/model"
	"github.com/Comcast/trickster/internal/timeseries"
)

// CAQLHandler handles CAQL requests for timeseries data and processes them
// through the delta proxy cache.
func (c *Client) CAQLHandler(w http.ResponseWriter, r *http.Request) {
	u := c.BuildUpstreamURL(r)
	engines.DeltaProxyCacheRequest(
		model.NewRequest("CAQLHandler",
			r.Method, u, r.Header, c.config.Timeout, r, c.webClient),
		w, c)
}

// caqlHandlerSetExtent will change the upstream request query to use the
// provided Extent.
func (c Client) caqlHandlerSetExtent(r *model.Request,
	extent *timeseries.Extent) {

	if r == nil || extent == nil || (extent.Start.IsZero() && extent.End.IsZero()) {
		return
	}

	trq := r.TimeRangeQuery
	var err error
	if trq == nil {
		if trq, err = c.ParseTimeRangeQuery(r); err != nil {
			return
		}
	}

	st := extent.Start.UnixNano() - (extent.Start.UnixNano() % int64(trq.Step))
	et := extent.End.UnixNano() - (extent.End.UnixNano() % int64(trq.Step))
	if st == et {
		et += int64(trq.Step)
	}

	q := r.URL.Query()
	q.Set(upCAQLStart, formatTimestamp(time.Unix(0, st), false))
	q.Set(upCAQLEnd, formatTimestamp(time.Unix(0, et), false))
	r.URL.RawQuery = q.Encode()
}

// caqlHandlerParseTimeRangeQuery parses the key parts of a TimeRangeQuery
// from the inbound HTTP Request.
func (c *Client) caqlHandlerParseTimeRangeQuery(
	r *model.Request) (*timeseries.TimeRangeQuery, error) {
	trq := &timeseries.TimeRangeQuery{}
	trq.Statement = r.URL.Path

	qp := r.URL.Query()
	var err error
	p := ""

	if p = qp.Get(upQuery); p == "" {
		if p = qp.Get(upCAQLQuery); p == "" {
			return nil, errors.MissingURLParam(upQuery + " or " + upCAQLQuery)
		}
	}

	trq.Statement = p

	if p = qp.Get(upCAQLStart); p == "" {
		return nil, errors.MissingURLParam(upCAQLStart)
	}

	if trq.Extent.Start, err = parseTimestamp(p); err != nil {
		return nil, err
	}

	if p = qp.Get(upCAQLEnd); p == "" {
		return nil, errors.MissingURLParam(upCAQLEnd)
	}

	if trq.Extent.End, err = parseTimestamp(p); err != nil {
		return nil, err
	}

	if p = qp.Get(upCAQLPeriod); p == "" {
		return nil, errors.MissingURLParam(upCAQLPeriod)
	}

	if !strings.HasSuffix(p, "s") {
		p += "s"
	}

	if trq.Step, err = parseDuration(p); err != nil {
		return nil, err
	}

	return trq, nil
}

// caqlHandlerFastForwardURL returns the url to fetch the Fast Forward value
// based on a timerange URL.
func (c *Client) caqlHandlerFastForwardURL(
	r *model.Request) (*url.URL, error) {
	var err error
	u := model.CopyURL(r.URL)
	q := u.Query()
	trq := r.TimeRangeQuery
	if trq == nil {
		trq, err = c.ParseTimeRangeQuery(r)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().Unix()
	start := now - (now % int64(trq.Step.Seconds()))
	end := start + int64(trq.Step.Seconds())
	q.Set(upCAQLStart, formatTimestamp(time.Unix(start, 0), false))
	q.Set(upCAQLEnd, formatTimestamp(time.Unix(end, 0), false))
	u.RawQuery = q.Encode()
	return u, nil
}