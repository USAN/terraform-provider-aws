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
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
)

func ResourceUsers() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceUserCreate,
		ReadContext:   resourceUserRead,
		UpdateContext: resourceUserUpdate,
		DeleteContext: resourceUserDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"directory_user_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"hierarchy_group_id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"identity_info": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"email": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"first_name": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"last_name": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"instance_id": {
				Type:         schema.TypeString,
				Required:     true,
			},
			"password": {
				Type:         schema.TypeString,
				Optional:     true,
			},
			"phone_config": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"after_contact_work_time_limit": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"auto_accept": {
							Type:     schema.TypeBool,
							Optional: true,
						},
						"desk_phone_number": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"phone_type": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{"SOFT_PHONE", "DESK_PHONE"}, false),
						},
					},
				},
			},			
			"routing_profile_id": {
				Type:         schema.TypeString,
				Required:     true,
			},
			"security_profile_ids": {
				Type:     schema.TypeSet,
				Required: true,
				MaxItems: 500,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
				},
			},
			"username": {
				Type:         schema.TypeString,
				Required:     true,
			},
			"user_id": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	log.Printf("[DEBUG] user gather input vars ( %s )", d)
	instanceID              := d.Get("instance_id").(string)
	identityInfo            := expandIdentityInfoConfig(d.Get("identity_info").([]interface{}))
	phoneConfig             := expandPhoneConfig(d.Get("phone_config").([]interface{}))
	routingProfileID        := d.Get("routing_profile_id").(string)
	securityProfileIDs      := flex.ExpandStringSet(d.Get("security_profile_ids").(*schema.Set))
	username                := d.Get("username").(string)
	log.Printf("[DEBUG] user set input vars ( %s )", d)

	input := &connect.CreateUserInput{
		InstanceId: aws.String(instanceID),
		IdentityInfo: identityInfo,
		PhoneConfig: phoneConfig,
		RoutingProfileId:  aws.String(routingProfileID),
		SecurityProfileIds: securityProfileIDs,
		Username: aws.String(username),
	}
	log.Printf("[DEBUG] user set password ( %s )", input)

	if v, ok := d.GetOk("password"); ok {
		input.Password = aws.String(v.(string))
	}
  log.Printf("[DEBUG] user set directory_user_id ( %s )", input)
	if v, ok := d.GetOk("directory_user_id"); ok {
		input.DirectoryUserId = aws.String(v.(string))
	}
	log.Printf("[DEBUG] user set hierarchy_group_id ( %s )", input)
	if v, ok := d.GetOk("hierarchy_group_id"); ok {
		input.HierarchyGroupId = aws.String(v.(string))
	}

	if len(tags) > 0 {
		input.Tags = Tags(tags.IgnoreAWS())
	}

	log.Printf("[DEBUG] Creating User %s", input)
	output, err := conn.CreateUserWithContext(ctx, input)

	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating User (%s): %w", username, err))
	}

	if output == nil {
		return diag.FromErr(fmt.Errorf("error creating User (%s): empty output", username))
	}
	log.Printf("[DEBUG] set useridr %s", input)
	d.SetId(fmt.Sprintf("%s:%s", instanceID, aws.StringValue(output.UserId)))

	return resourceUserRead(ctx, d, meta)
}

func resourceUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	instanceID, userID, err := ParseUserID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := conn.DescribeUserWithContext(ctx, &connect.DescribeUserInput{
		InstanceId: aws.String(instanceID),
		UserId:     aws.String(userID),
	})

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, connect.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Connect User (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting Connect User (%s): %w", d.Id(), err))
	}

	if resp == nil || resp.User == nil {
		return diag.FromErr(fmt.Errorf("error getting Connect User (%s): empty response", d.Id()))
	}

	if err := d.Set("identity_info", flattenIdentityInfoConfig(resp.User.IdentityInfo)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("phone_config", flattenPhoneConfig(resp.User.PhoneConfig)); err != nil {
		return diag.FromErr(err)
	}
	d.Set("arn", resp.User.Arn)
	d.Set("directory_user_id", resp.User.DirectoryUserId)
	d.Set("hierarchy_group_id", resp.User.HierarchyGroupId)
	d.Set("user_id", resp.User.Id)
	d.Set("routing_profile_id", resp.User.RoutingProfileId)
	d.Set("security_profile_ids", resp.User.SecurityProfileIds)
	d.Set("tags", resp.User.Tags )
	d.Set("username", resp.User.Username )

	tags := KeyValueTags(resp.User.Tags).IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return diag.FromErr(fmt.Errorf("error setting tags: %w", err))
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return diag.FromErr(fmt.Errorf("error setting tags_all: %w", err))
	}

	return nil
}

func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceID, userID, err := ParseUserID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	_, err = conn.DeleteUserWithContext(ctx, &connect.DeleteUserInput{
		InstanceId:           aws.String(instanceID),
		UserId:               aws.String(userID),
	})

	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting Connect User (%s): %w", d.Id(), err))
	}

	return nil
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceID, userID, err := QueueParseID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	// Users have 5 update APIs
	// UpdateUserHierarchyWithContext: Updates the hierarchy_group_id of the specified user.
	// UpdateUserIdentityInfoWithContext  : Updates the identity_info of the specified userr.
	// UpdateUserPhoneConfigWithContext : Updates the phone_config of the specified user.
	// UpdateUserRoutingProfileWithContext: Updates the routing_profile_id of the specified user.
	// UpdateUserSecurityProfilesWithContext: Updates the security_profile_ids of the specified user

	// updates to user hierarchy_group_id
	if d.HasChange("hierarchy_group_id") {
		input := &connect.UpdateUserHierarchyInput{
			InstanceId:         aws.String(instanceID),
			UserId:             aws.String(userID),
			HierarchyGroupId:   aws.String(d.Get("hierarchy_group_id").(string)),
		}
		_, err = conn.UpdateUserHierarchyWithContext(ctx, input)

		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error updating User Hierarchy Group ID (%s): %w", d.Id(), err))
		}
	}

	// updates to identity_info
	if d.HasChange("identity_info") {
		input := &connect.UpdateUserIdentityInfoInput{
			InstanceId:         aws.String(instanceID),
			UserId:             aws.String(userID),
			IdentityInfo:       expandIdentityInfoConfig(d.Get("identity_info").([]interface{})),
		}
		_, err = conn.UpdateUserIdentityInfoWithContext(ctx, input)

		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error updating User Identity Info (%s): %w", d.Id(), err))
		}
	}

	// updates to phone_config
	if d.HasChange("phone_config") {
		input := &connect.UpdateUserPhoneConfigInput{
			InstanceId:         aws.String(instanceID),
			UserId:             aws.String(userID),
			PhoneConfig:        expandPhoneConfig(d.Get("phone_config").([]interface{})),
		}
		_, err = conn.UpdateUserPhoneConfigWithContext(ctx, input)

		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error updating Queue Outbound Caller Config (%s): %w", d.Id(), err))
		}
	}

	// updates to routing_profile_id
	if d.HasChange("routing_profile_id") {
		input := &connect.UpdateUserRoutingProfileInput{
			InstanceId:         aws.String(instanceID),
			UserId:             aws.String(userID),
			RoutingProfileId:   aws.String(d.Get("routing_profile_id").(string)),
		}
		_, err = conn.UpdateUserRoutingProfileWithContext(ctx, input)

		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error updating User Routing Profile ID (%s): %w", d.Id(), err))
		}
	}

	// updates to security_profile_ids
	if d.HasChange("security_profile_ids") {
		input := &connect.UpdateUserSecurityProfilesInput{
			InstanceId:         aws.String(instanceID),
			UserId:             aws.String(userID),
			SecurityProfileIds: flex.ExpandStringSet(d.Get("security_profile_ids").(*schema.Set)),
		}
		_, err = conn.UpdateUserSecurityProfilesWithContext(ctx, input)

		if err != nil {
			return diag.FromErr(fmt.Errorf("[ERROR] Error updating User Security Profile ID (%s): %w", d.Id(), err))
		}
	}

	// updates to tags
	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")
		if err := UpdateTags(conn, d.Id(), o, n); err != nil {
			return diag.FromErr(fmt.Errorf("error updating tags: %w", err))
		}
	}

	return resourceUserRead(ctx, d, meta)
}

func expandIdentityInfoConfig(userIdentityInfo []interface{}) *connect.UserIdentityInfo {
	if len(userIdentityInfo) == 0 || userIdentityInfo[0] == nil {
		return nil
	}

	tfMap, ok := userIdentityInfo[0].(map[string]interface{})
	if !ok {
		return nil
	}

	result := &connect.UserIdentityInfo{}

	if v, ok := tfMap["email"].(string); ok && v != "" {
		result.Email = aws.String(v)
	}
	if v, ok := tfMap["first_name"].(string); ok && v != "" {
		result.FirstName = aws.String(v)
	}
	if v, ok := tfMap["last_name"].(string); ok && v != "" {
		result.LastName = aws.String(v)
	}

	return result
}

func expandPhoneConfig(userPhoneConfig  []interface{}) *connect.UserPhoneConfig {
	if len(userPhoneConfig) == 0 || userPhoneConfig[0] == nil {
		return nil
	}

	tfMap, ok := userPhoneConfig[0].(map[string]interface{})
	if !ok {
		return nil
	}

	result := &connect.UserPhoneConfig {}

	if v, ok := tfMap["after_contact_work_time_limit"].(int64); ok {
		result.AfterContactWorkTimeLimit = aws.Int64(int64(v))
	}
	if v, ok := tfMap["auto_accept"].(bool); ok {
		result.AutoAccept = aws.Bool(v)
	}
	if v, ok := tfMap["desk_phone_number"].(string); ok && v != "" {
		result.DeskPhoneNumber = aws.String(v)
	}
	if v, ok := tfMap["phone_type"].(string); ok && v != "" {
		result.PhoneType = aws.String(v)
	}

	return result
}

func flattenIdentityInfoConfig(identityInfoConfig *connect.UserIdentityInfo) []interface{} {
	if identityInfoConfig == nil {
		return []interface{}{}
	}

	values := map[string]interface{}{}

	if v := identityInfoConfig.Email; v != nil {
		values["email"] = aws.StringValue(v)
	}

	if v := identityInfoConfig.FirstName; v != nil {
		values["first_name"] = aws.StringValue(v)
	}

	if v := identityInfoConfig.LastName; v != nil {
		values["last_name"] = aws.StringValue(v)
	}

	return []interface{}{values}
}
func flattenPhoneConfig(phoneConfig *connect.UserPhoneConfig) []interface{} {
	if phoneConfig == nil {
		return []interface{}{}
	}

	values := map[string]interface{}{}

	if v := phoneConfig.AfterContactWorkTimeLimit; v != nil {
		values["after_contact_work_time_limit"] = aws.Int64Value(v)
	}
	if v := phoneConfig.AutoAccept; v != nil {
		values["auto_accept"] = aws.BoolValue(v)
	}
	if v := phoneConfig.DeskPhoneNumber; v != nil {
		values["desk_phone_number"] = aws.StringValue(v)
	}
	if v := phoneConfig.PhoneType; v != nil {
		values["phone_type"] = aws.StringValue(v)
	}

	return []interface{}{values}
}

func ParseUserID(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unexpected format of ID (%s), expected instanceID:userID", id)
	}

	return parts[0], parts[1], nil
}
