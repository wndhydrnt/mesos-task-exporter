package main

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRetrieveCurrentMasterState(t *testing.T) {
	var masterUrl *url.URL

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		master := Master{Leader: "master1@" + masterUrl.Host}

		data, _ := json.Marshal(master)

		w.Write(data)
		return
	}))

	masterUrl, _ = url.Parse(ts.URL)

	m := masterPoller{
		config: &Config{
			MesosMasters: []*url.URL{masterUrl},
		},
		httpClient: &http.Client{},
	}

	res, _ := m.retrieveCurrentMasterState()

	require.Equal(t, "master1@"+masterUrl.Host, res.Leader)
	require.Equal(t, masterUrl, m.currentMesosMaster)
}

func TestChangingMesosMasterLeader(t *testing.T) {
	var firstMasterUrl *url.URL
	var secondMasterUrl *url.URL

	firstMasterReqCount := 0
	secondMasterReqCount := 0

	firstMaster := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var master Master

		if firstMasterReqCount == 0 {
			master = Master{Leader: "master1@" + firstMasterUrl.Host}
		} else {
			master = Master{Leader: "master2@" + secondMasterUrl.Host}
		}

		firstMasterReqCount = firstMasterReqCount + 1

		data, _ := json.Marshal(master)

		w.Write(data)
		return
	}))

	secondMaster := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		master := Master{Leader: "master2@" + secondMasterUrl.Host}

		data, _ := json.Marshal(master)

		secondMasterReqCount = secondMasterReqCount + 1

		w.Write(data)
		return
	}))

	firstMasterUrl, _ = url.Parse(firstMaster.URL)
	secondMasterUrl, _ = url.Parse(secondMaster.URL)

	m := masterPoller{
		config: &Config{
			MesosMasters: []*url.URL{firstMasterUrl, secondMasterUrl},
		},
		httpClient: &http.Client{},
	}

	m.retrieveCurrentMasterState()
	res, _ := m.retrieveCurrentMasterState()

	require.Equal(t, "master2@"+secondMasterUrl.Host, res.Leader)
	require.Equal(t, secondMasterUrl, m.currentMesosMaster)
	require.Equal(t, 3, firstMasterReqCount)
	require.Equal(t, 1, secondMasterReqCount)
}
