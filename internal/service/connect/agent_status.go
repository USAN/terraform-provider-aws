package connect

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/connect"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
)

func ResourceAgentStatus() *schema.Resource {
	log.Printf("[KEEGAN] agent_status.go")
	return &schema.Resource{
		CreateContext: resourceAgentStatusCreate,
		ReadContext:   resourceAgentStatusRead,
		UpdateContext: resourceAgentStatusUpdate,
		DeleteContext: resourceAgentStatusDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringLenBetween(1, 250),
			},
			"agent_status_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"instance_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(1, 127),
			},
			"state": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"ENABLED", "DISABLED"}, false),
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
		},
	}
}

func resourceAgentStatusCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	instanceID := d.Get("instance_id").(string)
	name := d.Get("name").(string)

	input := &connect.CreateAgentStatusInput{
		InstanceId: aws.String(instanceID),
		Name:       aws.String(name),
		State:      aws.String(d.Get("state").(string)),
	}

	if v, ok := d.GetOk("description"); ok {
		input.Description = aws.String(v.(string))
	}

	if len(tags) > 0 {
		input.Tags = Tags(tags.IgnoreAWS())
	}

	log.Printf("[DEBUG] Creating Connect Agent Status %s", input)
	output, err := conn.CreateAgentStatusWithContext(ctx, input)

	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating Connect Agent Status (%s): %w", name, err))
	}

	if output == nil {
		return diag.FromErr(fmt.Errorf("error creating Connect Agent Status (%s): empty output", name))
	}

	d.SetId(fmt.Sprintf("%s:%s", instanceID, aws.StringValue(output.AgentStatusId)))

	return resourceAgentStatusRead(ctx, d, meta)
}

func resourceAgentStatusRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	instanceID, agentStatusID, err := AgentStatusParseID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := conn.DescribeAgentStatusWithContext(ctx, &connect.DescribeAgentStatusInput{
		AgentStatusId: aws.String(agentStatusID),
		InstanceId:    aws.String(instanceID),
	})

	if !d.IsNewResource() && tfawserr.ErrMessageContains(err, connect.ErrCodeResourceNotFoundException, "") {
		log.Printf("[WARN] Connect Agent Status (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting Connect Agent Status (%s): %w", d.Id(), err))
	}

	if resp == nil || resp.AgentStatus == nil {
		return diag.FromErr(fmt.Errorf("error getting Connect Agent Status (%s): empty response", d.Id()))
	}

	//if err := d.Set("config", flattenAgentStatus(resp.AgentStatus.Config)); err != nil {
	//	return diag.FromErr(err)
	//}

	d.Set("arn", resp.AgentStatus.AgentStatusARN)
	d.Set("agent_status_arn", resp.AgentStatus.AgentStatusARN) // Deprecated
	d.Set("agent_status_id", resp.AgentStatus.AgentStatusId)
	d.Set("instance_id", instanceID)
	d.Set("description", resp.AgentStatus.Description)
	d.Set("name", resp.AgentStatus.Name)
	d.Set("state", resp.AgentStatus.State)
	d.Set("type", resp.AgentStatus.Type)

	tags := KeyValueTags(resp.AgentStatus.Tags).IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return diag.FromErr(fmt.Errorf("error setting tags: %w", err))
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return diag.FromErr(fmt.Errorf("error setting tags_all: %w", err))
	}

	return nil
}

func resourceAgentStatusUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceID, agentStatusID, err := AgentStatusParseID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	input := &connect.UpdateAgentStatusInput{
		AgentStatusId: aws.String(agentStatusID),
		InstanceId:    aws.String(instanceID),
	}

	//if d.HasChange("config") {
	//	config := expandAgentStatus(d.Get("config").(*schema.Set).List())
	//	input.Config = config
	//}

	if d.HasChange("name") {
		input.Name = aws.String(d.Get("name").(string))
	}

	if d.HasChange("description") {
		input.Description = aws.String(d.Get("description").(string))
	}

	if d.HasChange("state") {
		input.State = aws.String(d.Get("state").(string))
	}

	if d.HasChange("type") {
		input.State = aws.String(d.Get("type").(string))
	}

	_, err = conn.UpdateAgentStatusWithContext(ctx, input)

	if err != nil {
		return diag.FromErr(fmt.Errorf("[ERROR] Error updating Agent Status (%s): %w", d.Id(), err))
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")
		if err := UpdateTags(conn, d.Id(), o, n); err != nil {
			return diag.FromErr(fmt.Errorf("error updating tags: %w", err))
		}
	}

	return resourceAgentStatusRead(ctx, d, meta)
}

func resourceAgentStatusDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	//conn := meta.(*conns.AWSClient).ConnectConn

	//instanceID, agentStatusID, err := AgentStatusParseID(d.Id())
	//
	//	if err != nil {
	//		return diag.FromErr(err)
	//	}
	//
	//	_, err = conn.DeleteAgentStatusWithContext(ctx, &connect.DeleteAgentStatusInput{
	//		AgentStatusId: aws.String(agentStatusID),
	//		InstanceId:    aws.String(instanceID),
	//	})
	//
	//	if err != nil {
	//		return diag.FromErr(fmt.Errorf("error deleting AgentStatus (%s): %w", d.Id(), err))
	//	}
	//
	return nil
}

//func expandAgentStatus(configs []interface{}) []*connect.AgentStatusConfig {
//	if len(configs) == 0 {
//		return nil
//	}
//
//	agentStatusConfigs := []*connect.AgentStatusConfig{}
//	for _, config := range configs {
//		data := config.(map[string]interface{})
//		agentStatusConfig := &connect.AgentStatusConfig{
//			Name:  aws.String(data["name"].(string)),
//			State: aws.String(data["state"].(string)),
//		}
//
//		agentStatusConfigs = append(agentStatusConfigs, agentStatusConfig)
//	}
//
//	return agentStatusConfigs
//}

//func flattenAgentStatus(configs []*connect.AgentStatusConfig) []interface{} {
//	configsList := []interface{}{}
//	for _, config := range configs {
//		values := map[string]interface{}{}
//		values["name"] = aws.StringValue(config.Name)
//		values["state"] = aws.StringValue(config.State)
//
//		configsList = append(configsList, values)
//	}
//	return configsList
//}

func AgentStatusParseID(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%s), expected instanceID:agentStatusID", id)
	}

	return parts[0], parts[1], nil
}
