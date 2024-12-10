package driver

import (
	"reflect"
	"testing"

	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
)

func TestDriver_getDBInfo(t *testing.T) {
	type fields struct {
		lc        logger.LoggingClient
		asyncCh   chan<- *sdkModel.AsyncValues
		s7Clients map[string]*S7Client
	}
	type args struct {
		variable string
	}
	var driver fields
	driver.lc = logger.NewClient("S7", "Error")
	tests := []struct {
		name       string
		fields     *fields
		args       args
		wantDbInfo *DBInfo
		wantErr    bool
	}{
		{
			name:       "invalid address-DB1.DBX100",
			fields:     &driver,
			args:       args{variable: "DB1.DBX100"},
			wantDbInfo: nil,
			wantErr:    true,
		},
		{
			name:       "invalid address-DB1.DBX86.2.1",
			fields:     &driver,
			args:       args{variable: "DB1.DBX86.2.1"},
			wantDbInfo: nil,
			wantErr:    true,
		},
		{
			name:       "invalid address-DBX100",
			fields:     &driver,
			args:       args{variable: "DBX100"},
			wantDbInfo: nil,
			wantErr:    true,
		},
		{
			name:   "valid address-DB1.DBX100.0",
			fields: &driver,
			args:   args{variable: "DB1.DBX100.0"},
			wantDbInfo: &DBInfo{
				Area:       0x84,
				DBNumber:   1,
				Start:      800,
				Amount:     1,
				WordLength: s7wlbit,
				DBArray:    []string{"DB1", "DBX100", "0"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Driver{
				lc:        tt.fields.lc,
				asyncCh:   tt.fields.asyncCh,
				s7Clients: tt.fields.s7Clients,
			}
			gotDbInfo, err := s.getDBInfo(tt.args.variable)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDBInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotDbInfo, tt.wantDbInfo) {
				t.Errorf("getDBInfo() gotDbInfo = %v, want %v", gotDbInfo, tt.wantDbInfo)
			}
		})
	}
}
