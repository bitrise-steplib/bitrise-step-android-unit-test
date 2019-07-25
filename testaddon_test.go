package main

import (
	"testing"
)

func TestGetVariantDir(t *testing.T) {
	tc := []struct{
		title string
		path string
		wantStr string
		isErr bool
	}{
		{
			title: "should return error on empty string",
			path: "",
			wantStr: "",
			isErr: true, 
		},
		{
			title: "should return error if artifact path ends in test results folder with trailing slash",
			path: "/path/to/test-results/",
			wantStr: "",
			isErr: true, 
		},
	}
	
	for _, tt := range tc {
		str, err := getVariantDir(tt.path)
		if str != tt.wantStr || (err != nil) != tt.isErr {
			t.Fatalf("%s: got (%s, %s)", tt.title, str, err)
		}
	}

}