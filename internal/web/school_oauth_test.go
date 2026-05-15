package web

import "testing"

func TestExtractOAuthCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   schoolAuthInput
		want string
	}{
		{
			name: "callback url",
			in: schoolAuthInput{
				CallbackURL: "https://xhbcs.henau.edu.cn/?code=001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy&state=STATE#/checkin",
			},
			want: "001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy",
		},
		{
			name: "raw code",
			in: schoolAuthInput{
				OAuthCode: "001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy",
			},
			want: "001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy",
		},
		{
			name: "query only",
			in: schoolAuthInput{
				CallbackURL: "?code=001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy&state=STATE",
			},
			want: "001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractOAuthCode(tt.in)
			if err != nil {
				t.Fatalf("extractOAuthCode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("extractOAuthCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractOAuthCodeMissingCode(t *testing.T) {
	t.Parallel()

	if _, err := extractOAuthCode(schoolAuthInput{
		CallbackURL: "https://xhbcs.henau.edu.cn/#/checkin",
	}); err == nil {
		t.Fatal("extractOAuthCode() expected error, got nil")
	}
}
