package parser

import (
	"testing"
)

func TestParseChanges(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantAdd int
		wantChg int
		wantDes int
		wantNil bool
	}{
		{
			name:    "apply complete",
			output:  "Apply complete! Resources: 3 added, 1 changed, 0 destroyed.",
			wantAdd: 3,
			wantChg: 1,
			wantDes: 0,
		},
		{
			name:    "plan changes",
			output:  "Plan: 2 to add, 0 to change, 1 to destroy.",
			wantAdd: 2,
			wantChg: 0,
			wantDes: 1,
		},
		{
			name:    "destroy complete",
			output:  "Destroy complete! Resources: 5 destroyed.",
			wantAdd: 0,
			wantChg: 0,
			wantDes: 5,
		},
		{
			name:    "no changes",
			output:  "No changes. Your infrastructure matches the configuration.",
			wantAdd: 0,
			wantChg: 0,
			wantDes: 0,
		},
		{
			name:    "no match",
			output:  "Some random terraform output",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.output)
			if tt.wantNil {
				if result.Changes != nil {
					t.Errorf("expected nil Changes, got %+v", result.Changes)
				}
				return
			}

			if result.Changes == nil {
				t.Fatal("expected non-nil Changes")
			}

			if result.Changes.Add != tt.wantAdd {
				t.Errorf("Add = %d, want %d", result.Changes.Add, tt.wantAdd)
			}
			if result.Changes.Change != tt.wantChg {
				t.Errorf("Change = %d, want %d", result.Changes.Change, tt.wantChg)
			}
			if result.Changes.Destroy != tt.wantDes {
				t.Errorf("Destroy = %d, want %d", result.Changes.Destroy, tt.wantDes)
			}
		})
	}
}

func TestParseResources(t *testing.T) {
	output := `aws_instance.web: Creating...
aws_instance.web: Creation complete after 45s [id=i-abc123]
aws_security_group.allow_ssh: Modifying... [id=sg-xyz789]
aws_security_group.allow_ssh: Modification complete after 2s [id=sg-xyz789]`

	result := Parse(output)

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	if result.Resources[0].Address != "aws_instance.web" {
		t.Errorf("first resource address = %s, want aws_instance.web", result.Resources[0].Address)
	}
	if result.Resources[0].Action != "create" {
		t.Errorf("first resource action = %s, want create", result.Resources[0].Action)
	}
	if result.Resources[0].Status != "success" {
		t.Errorf("first resource status = %s, want success", result.Resources[0].Status)
	}

	if result.Resources[1].Address != "aws_security_group.allow_ssh" {
		t.Errorf("second resource address = %s, want aws_security_group.allow_ssh", result.Resources[1].Address)
	}
	if result.Resources[1].Action != "update" {
		t.Errorf("second resource action = %s, want update", result.Resources[1].Action)
	}
}
