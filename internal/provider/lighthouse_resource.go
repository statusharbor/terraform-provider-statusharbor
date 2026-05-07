package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/statusharbor/terraform-provider-statusharbor/internal/client"
)

var _ resource.Resource = (*lighthouseResource)(nil)
var _ resource.ResourceWithImportState = (*lighthouseResource)(nil)
var _ resource.ResourceWithConfigure = (*lighthouseResource)(nil)

type lighthouseResource struct {
	c *client.Client
}

// NewLighthouseResource is the constructor used by the framework.
func NewLighthouseResource() resource.Resource {
	return &lighthouseResource{}
}

func (r *lighthouseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lighthouse"
}

func (r *lighthouseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Status Harbor Lighthouse — a private-network monitoring agent " +
			"registered to your team. Apply mints the agent's bearer token (sensitive output); " +
			"the agent itself is deployed separately via the terraform-lighthouse modules, " +
			"a Helm chart, install.sh, or `docker run`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the Lighthouse, assigned by the server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Display name of the Lighthouse. Must be unique within the team. " +
					"Renaming triggers an in-place update.",
			},
			"host": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Optional hostname hint shown in the dashboard until the agent " +
					"registers and reports its real hostname. Once the agent registers, the " +
					"server overwrites this with agent_hostname; the field is then read-only " +
					"in practice. Leave empty to defer to the agent.",
			},
			"notify_on_lifecycle": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the team is paged when this Lighthouse goes online / " +
					"offline. Defaults to true server-side.",
			},
			"flap_protection_threshold": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "How many consecutive failures the agent must observe before " +
					"reporting a state transition. Server default applies if omitted.",
			},
			"paused": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "When true, the agent runs no checks. Use to pause monitoring " +
					"during planned maintenance.",
			},
			"agent_hostname": schema.StringAttribute{
				Computed: true,
				Description: "Hostname the agent reported on its last register/heartbeat. " +
					"Read-only; reflects the actual machine running the agent.",
			},
			"agent_version": schema.StringAttribute{
				Computed:    true,
				Description: "Agent binary version reported on the last register/heartbeat.",
			},
			"last_heartbeat_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC3339 timestamp of the most recent heartbeat. Empty until the agent connects.",
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
			"updated_at": schema.StringAttribute{
				Computed: true,
			},
			"token": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				Description: "Bearer token the agent uses to authenticate. Returned ONLY on " +
					"resource creation; subsequent reads echo the same value from state. " +
					"Persists in your state file — use a remote, encrypted state backend " +
					"(Terraform Cloud, S3+KMS, GCS+KMS, etc.).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *lighthouseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.c = c
}

// lighthouseModel mirrors the resource schema. Pointers / NullX
// types because the server may return null on optional fields.
type lighthouseModel struct {
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
	Token                   types.String `tfsdk:"token"`
}

func (r *lighthouseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lighthouseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := client.CreateLighthouseRequest{Name: plan.Name.ValueString()}
	if !plan.Host.IsNull() && !plan.Host.IsUnknown() {
		v := plan.Host.ValueString()
		createReq.Host = &v
	}

	created, err := r.c.CreateLighthouse(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("create lighthouse failed", err.Error())
		return
	}

	// Some optional fields (notify_on_lifecycle, flap_protection_threshold,
	// paused) take server-side defaults on create, then can be set via
	// PATCH. Apply user-supplied non-default values in a single follow-up
	// PATCH so the next plan has nothing to do.
	needsPatch := false
	updateReq := client.UpdateLighthouseRequest{}
	if !plan.NotifyOnLifecycle.IsNull() && !plan.NotifyOnLifecycle.IsUnknown() &&
		plan.NotifyOnLifecycle.ValueBool() != created.NotifyOnLifecycle {
		v := plan.NotifyOnLifecycle.ValueBool()
		updateReq.NotifyOnLifecycle = &v
		needsPatch = true
	}
	if !plan.FlapProtectionThreshold.IsNull() && !plan.FlapProtectionThreshold.IsUnknown() &&
		plan.FlapProtectionThreshold.ValueInt64() != int64(created.FlapProtectionThreshold) {
		v := int(plan.FlapProtectionThreshold.ValueInt64())
		updateReq.FlapProtectionThreshold = &v
		needsPatch = true
	}
	if !plan.Paused.IsNull() && !plan.Paused.IsUnknown() &&
		plan.Paused.ValueBool() != created.Paused {
		v := plan.Paused.ValueBool()
		updateReq.Paused = &v
		needsPatch = true
	}
	final := &created.Lighthouse
	if needsPatch {
		updated, err := r.c.UpdateLighthouse(ctx, created.ID, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("apply post-create patch failed", err.Error())
			return
		}
		final = updated
	}

	state := buildModel(final)
	state.Token = types.StringValue(created.Token)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lighthouseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lighthouseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lh, err := r.c.GetLighthouse(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			// Deleted out-of-band — drop from state so plan reconciles.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read lighthouse failed", err.Error())
		return
	}

	// Token isn't returned by GET — preserve whatever's in state.
	prevToken := state.Token
	state = buildModel(lh)
	state.Token = prevToken

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lighthouseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lighthouseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Renames need their own endpoint (not exposed via PATCH; use the
	// dashboard). Refuse for now and tell the user.
	if !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddError("renaming a Lighthouse is not supported",
			"The Status Harbor API doesn't expose rename via the standard PATCH endpoint. "+
				"Rename the Lighthouse in the Console UI and re-run terraform import to refresh state.")
		return
	}

	updateReq := client.UpdateLighthouseRequest{}
	if !plan.Host.Equal(state.Host) && !plan.Host.IsUnknown() {
		v := plan.Host.ValueString()
		updateReq.Host = &v
	}
	if !plan.NotifyOnLifecycle.Equal(state.NotifyOnLifecycle) && !plan.NotifyOnLifecycle.IsUnknown() {
		v := plan.NotifyOnLifecycle.ValueBool()
		updateReq.NotifyOnLifecycle = &v
	}
	if !plan.FlapProtectionThreshold.Equal(state.FlapProtectionThreshold) && !plan.FlapProtectionThreshold.IsUnknown() {
		v := int(plan.FlapProtectionThreshold.ValueInt64())
		updateReq.FlapProtectionThreshold = &v
	}
	if !plan.Paused.Equal(state.Paused) && !plan.Paused.IsUnknown() {
		v := plan.Paused.ValueBool()
		updateReq.Paused = &v
	}

	updated, err := r.c.UpdateLighthouse(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("update lighthouse failed", err.Error())
		return
	}

	prevToken := state.Token
	newState := buildModel(updated)
	newState.Token = prevToken

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *lighthouseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lighthouseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.c.DeleteLighthouse(ctx, state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("delete lighthouse failed", err.Error())
		return
	}
}

func (r *lighthouseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildModel(lh *client.Lighthouse) lighthouseModel {
	return lighthouseModel{
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
}

func nullableString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
