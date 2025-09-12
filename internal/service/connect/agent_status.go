package connect

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/connect"
	awstypes "github.com/aws/aws-sdk-go-v2/service/connect/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_connect_agent_status", name="Agent Status")
// @Tags(identifierAttribute="arn")
func ResourceAgentStatus() *schema.Resource {
	log.Printf("[KEEGAN] agent_status.go")
	return &schema.Resource{
		CreateContext: resourceAgentStatusCreate,
		ReadContext:   resourceAgentStatusRead,
		UpdateContext: resourceAgentStatusUpdate,
		// Agent Status does not support deletion today. NoOp the Delete method.
		// Users can rename their Agent Status manually if they want.
		DeleteContext: schema.NoopContext,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrDescription: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 250),
			},
			"agent_status_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrInstanceID: {
				Type:     schema.TypeString,
				Required: true,
			},
			names.AttrName: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 127),
			},
			names.AttrState: {
				Type:             schema.TypeString,
				Required:         true,
				ValidateDiagFunc: enum.Validate[awstypes.AgentStatusState](),
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
		},
	}
}

func resourceAgentStatusCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ConnectClient(ctx)

	instanceID := d.Get(names.AttrInstanceID).(string)
	name := d.Get(names.AttrName).(string)
	input := &connect.CreateAgentStatusInput{
		InstanceId: aws.String(instanceID),
		Name:       aws.String(name),
		State:      awstypes.AgentStatusState(d.Get(names.AttrState).(string)),
		Tags:       getTagsIn(ctx),
	}

	if v, ok := d.GetOk(names.AttrDescription); ok {
		input.Description = aws.String(v.(string))
	}

	output, err := conn.CreateAgentStatus(ctx, input)

	if err != nil {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("error creating Connect Agent Status (%s): %s", name, err))
	}

	if output == nil {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("error creating Connect Agent Status (%s): empty output", name))
	}

	d.SetId(fmt.Sprintf("%s:%s", instanceID, aws.ToString(output.AgentStatusId)))

	return append(diags, resourceAgentStatusRead(ctx, d, meta)...)
}

func resourceAgentStatusRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ConnectClient(ctx)

	instanceID, agentStatusID, err := AgentStatusParseID(d.Id())

	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	resp, err := conn.DescribeAgentStatus(ctx, &connect.DescribeAgentStatusInput{
		AgentStatusId: aws.String(agentStatusID),
		InstanceId:    aws.String(instanceID),
	})

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Connect Agent Status (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("error getting Connect Agent Status (%s): %s", d.Id(), err))
	}

	if resp == nil || resp.AgentStatus == nil {
		return sdkdiag.AppendFromErr(diags, fmt.Errorf("error getting Connect Agent Status (%s): empty response", d.Id()))
	}

	d.Set(names.AttrARN, resp.AgentStatus.AgentStatusARN)
	d.Set("agent_status_id", resp.AgentStatus.AgentStatusId)
	d.Set(names.AttrInstanceID, instanceID)
	d.Set(names.AttrDescription, resp.AgentStatus.Description)
	d.Set(names.AttrName, resp.AgentStatus.Name)
	d.Set(names.AttrState, resp.AgentStatus.State)
	d.Set("type", resp.AgentStatus.Type)

	setTagsOut(ctx, resp.AgentStatus.Tags)

	return diags
}

func resourceAgentStatusUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).ConnectClient(ctx)

	instanceID, agentStatusID, err := AgentStatusParseID(d.Id())
	if err != nil {
		return sdkdiag.AppendFromErr(diags, err)
	}

	if d.HasChanges(names.AttrName, names.AttrDescription, names.AttrState) {
		input := &connect.UpdateAgentStatusInput{
			AgentStatusId: aws.String(agentStatusID),
			InstanceId:    aws.String(instanceID),
			Name:          aws.String(d.Get(names.AttrName).(string)),
			Description:   aws.String(d.Get(names.AttrDescription).(string)),
			State:         awstypes.AgentStatusState(d.Get(names.AttrState).(string)),
		}

		_, err = conn.UpdateAgentStatus(ctx, input)

		if err != nil {
			return sdkdiag.AppendErrorf(diags, "[ERROR] Error updating Agent Status (%s): %s", d.Id(), err)
		}
	}

	return append(diags, resourceAgentStatusRead(ctx, d, meta)...)
}

func AgentStatusParseID(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%s), expected instanceID:agentStatusID", id)
	}

	return parts[0], parts[1], nil
}
