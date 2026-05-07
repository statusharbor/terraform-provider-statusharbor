package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/statusharbor/terraform-provider-statusharbor/internal/client"
)

var _ datasource.DataSource = (*lighthouseDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*lighthouseDataSource)(nil)

type lighthouseDataSource struct {
	c *client.Client
}

// NewLighthouseDataSource is a read-only lookup by id. Useful for
// modules that take a Lighthouse as input — they accept the id, the
// data source resolves the metadata at plan time.
func NewLighthouseDataSource() datasource.DataSource {
	return &lighthouseDataSource{}
}

func (d *lighthouseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lighthouse"
}

func (d *lighthouseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up an existing Lighthouse by id (UUID).",
		Attributes: map[string]schema.Attribute{
			"id":                        schema.StringAttribute{Required: true},
			"name":                      schema.StringAttribute{Computed: true},
			"host":                      schema.StringAttribute{Computed: true},
			"notify_on_lifecycle":       schema.BoolAttribute{Computed: true},
			"flap_protection_threshold": schema.Int64Attribute{Computed: true},
			"paused":                    schema.BoolAttribute{Computed: true},
			"agent_hostname":            schema.StringAttribute{Computed: true},
			"agent_version":             schema.StringAttribute{Computed: true},
			"last_heartbeat_at":         schema.StringAttribute{Computed: true},
			"created_at":                schema.StringAttribute{Computed: true},
			"updated_at":                schema.StringAttribute{Computed: true},
		},
	}
}

func (d *lighthouseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.c = c
}

// lighthouseDataModel mirrors the data-source schema. Same shape as
// the resource model minus the write-only fields (token).
type lighthouseDataModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Host                    types.String `tfsdk:"host"`
	NotifyOnLifecycle       types.Bool   `tfsdk:"notify_on_lifecycle"`
	FlapProtectionThreshold types.Int64  `tfsdk:"flap_protection_threshold"`
	Paused                  types.Bool   `tfsdk:"paused"`
	AgentHostname           types.String `tfsdk:"agent_hostname"`
	AgentVersion            types.String `tfsdk:"agent_version"`
	LastHeartbeatAt         types.String `tfsdk:"last_heartbeat_at"`
	CreatedAt               types.String `tfsdk:"created_at"`
	UpdatedAt               types.String `tfsdk:"updated_at"`
}

func (d *lighthouseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg lighthouseDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lh, err := d.c.GetLighthouse(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("lookup failed", err.Error())
		return
	}

	cfg = lighthouseDataModel{
		ID:                      types.StringValue(lh.ID),
		Name:                    types.StringValue(lh.Name),
		Host:                    nullableString(lh.Host),
		NotifyOnLifecycle:       types.BoolValue(lh.NotifyOnLifecycle),
		FlapProtectionThreshold: types.Int64Value(int64(lh.FlapProtectionThreshold)),
		Paused:                  types.BoolValue(lh.Paused),
		AgentHostname:           nullableString(lh.AgentHostname),
		AgentVersion:            nullableString(lh.AgentVersion),
		LastHeartbeatAt:         nullableString(lh.LastHeartbeatAt),
		CreatedAt:               types.StringValue(lh.CreatedAt),
		UpdatedAt:               types.StringValue(lh.UpdatedAt),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
