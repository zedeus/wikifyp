package config

import "testing"

func TestFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "defaults",
			env:  nil,
			want: Config{Addr: ":8080", DatabaseURL: ""},
		},
		{
			name: "overrides",
			env: map[string]string{
				"WIKIFYP_ADDR":         ":9000",
				"WIKIFYP_DATABASE_URL": "postgres://localhost/wikifyp",
			},
			want: Config{Addr: ":9000", DatabaseURL: "postgres://localhost/wikifyp"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear first so ambient variables don't leak into the test.
			t.Setenv("WIKIFYP_ADDR", "")
			t.Setenv("WIKIFYP_DATABASE_URL", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := FromEnv()
			if got != tt.want {
				t.Errorf("FromEnv() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
