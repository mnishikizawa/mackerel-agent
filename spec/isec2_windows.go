package spec

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Songmu/retry"
	"github.com/StackExchange/wmi"
)

// Win32ComputerSystemProduct is struct for WMI. SKUNumber is nil-able.
// The fields except UUID are ommited to not be checked.
type Win32ComputerSystemProduct struct {
	// Caption           string
	// Description       string
	// IdentifyingNumber string
	// Name              string
	// SKUNumber         *string
	UUID string
	// Vendor            string
	// Version           string
}

// If the OS is Windows, check UUID in WMI class Win32_ComputerSystemProduct first. If UUID seems to be EC2-ish, call the metadata API (up to 3 times).
// ref. https://docs.aws.amazon.com/AWSEC2/latest/WindowsGuide/identify_ec2_instances.html
func isEC2(ctx context.Context) bool {
	var records []Win32ComputerSystemProduct
	err := wmi.Query("SELECT UUID FROM Win32_ComputerSystemProduct", &records)
	if err != nil {
		return false
	}
	if len(records) == 0 {
		return false
	}
	return isEC2WithSpecifiedWmiRecords(ctx, records)
}

func isEC2WithSpecifiedWmiRecords(ctx context.Context, records []Win32ComputerSystemProduct) bool {
	looksLikeEC2 := false
	for _, r := range records {
		if isEC2UUID(r.UUID) {
			looksLikeEC2 = true
			break
		}
	}
	if !looksLikeEC2 {
		return false
	}

	// give up if ctx already closed
	select {
	case <-ctx.Done():
		return false
	default:
	}

	res := false
	cl := httpCli()
	err := retry.WithContext(ctx, 3, 2*time.Second, func() error {
		// '/ami-id` is probably an AWS specific URL
		req, err := http.NewRequest("GET", ec2BaseURL.String()+"/latest/metadata/ami-id", nil)
		if err != nil {
			return nil // something wrong. give up
		}
		resp, err := cl.Do(req.WithContext(ctx))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		res = resp.StatusCode == 200
		return nil
	})

	if err == nil {
		return res
	}

	return false
}

func isEC2UUID(uuid string) bool {
	conds := func(uuid string) bool {
		if strings.HasPrefix(uuid, "ec2") || strings.HasPrefix(uuid, "EC2") {
			return true
		}
		return false
	}

	if conds(uuid) {
		return true
	}

	// Check as little endian.
	// see. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/identify_ec2_instances.html
	fields := strings.Split(uuid, "-")
	decoded, _ := hex.DecodeString(fields[0]) // fields[0]: UUID time_low(uint32)
	r := bytes.NewReader(decoded)
	var data uint32
	binary.Read(r, binary.LittleEndian, &data)

	return conds(fmt.Sprintf("%x", data))
}
