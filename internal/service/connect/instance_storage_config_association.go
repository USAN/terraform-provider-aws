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
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourceInstanceStorageConfigAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceInstanceStorageConfigAssociationCreate,
		ReadContext:   resourceInstanceStorageConfigAssociationRead,
		DeleteContext: resourceInstanceStorageConfigAssociationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"instance_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"resource_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice(connect.InstanceStorageResourceType_Values(), false), // Valid values: AGENT_EVENTS | CALL_RECORDINGS | CHAT_TRANSCRIPTS | CONTACT_TRACE_RECORDS | MEDIA_STREAMS | SCHEDULED_REPORTS | REAL_TIME_CONTACT_ANALYSIS_SEGMENTS
			},
			"storage_config": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"association_id": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"storage_type": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice(connect.StorageType_Values(), false), // Valid values: KINESIS_FIREHOSE | KINESIS_STREAM | KINESIS_VIDEO_STREAM | S3
						},
						"s3_config": {
							Type:     schema.TypeList,
				      Optional: true,
				      Elem: &schema.Resource{
					      Schema: map[string]*schema.Schema{
						      "bucket_name": {
							      Type:     schema.TypeString,
							      Required: true,
						      },
						      "bucket_prefix": {
						      	Type:     schema.TypeString,
						      	Required: true,
						      },
						      "encryption_config": {
										Type:     schema.TypeList,
										Required: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"encryption_type": {
													Type:     schema.TypeString,
													Required: true,
													ValidateFunc: validation.StringInSlice(connect.EncryptionType_Values (), false), // Valid values: KMS 
												},
												"key_id": {
													Type:     schema.TypeString,
													Required: true,
												},
						          },
					          },
				          },
			          },
							},
						},
						"kinesis_video_stream_config": {
							Type:     schema.TypeList,
				      Optional: true,
			        MaxItems: 1,
				      Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
					        "encryption_config": {
								  	Type:     schema.TypeList,
								  	Required: true,
								  	MaxItems: 1,
								  	Elem: &schema.Resource{
								  		Schema: map[string]*schema.Schema{
								  			"encryption_type": {
								  				Type:     schema.TypeString,
								  				Required: true,
								  				ValidateFunc: validation.StringInSlice(connect.EncryptionType_Values (), false), // Valid values: KMS 
								  			},
								  			"key_id": {
								  				Type:     schema.TypeString,
								  				Required: true,
								  			},
								  		},
								  	},
								  },
						      "prefix": {
						       	Type:     schema.TypeString,
						       	Required: true,
						      },
						      "retention_period_hours": {
								  	Type:     schema.TypeInt,
								  	Required: true,
				          },
								},
							},
						},
						"kinesis_stream_config": {
							Type:     schema.TypeList,
				      Optional: true,
			        MaxItems: 1,
				      Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
					        "stream_arn": {
									  Type:     schema.TypeString,
									  Required: true,
								  },
								},
							},
						},
						"kinesis_firehose_config": {
							Type:     schema.TypeList,
				      Optional: true,
			        MaxItems: 1,
				      Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
					        "firehose_arn": {
									  Type:     schema.TypeString,
									  Required: true,
								  },
								},
							},
						},
					},
				},
			},
		},
	}
}

func resourceInstanceStorageConfigAssociationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceId := d.Get("instance_id").(string)
	storageConfig := expandInstanceStorageConfig(d.Get("storage_config").([]interface{}))
	resourceType := d.Get("resource_type").(string)
	input := &connect.AssociateInstanceStorageConfigInput{
		InstanceId: aws.String(instanceId),
    StorageConfig: storageConfig,
		ResourceType: aws.String(resourceType),
	}

	output, err := conn.AssociateInstanceStorageConfigWithContext(ctx, input)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating Instance Storage Config Association (%s): %w", resourceType, err))
	}
	
   d.SetId(fmt.Sprintf("%s:%s:%s", instanceId, resourceType, aws.StringValue(output.AssociationId)))

	return resourceInstanceStorageConfigAssociationRead(ctx, d, meta)
}

func resourceInstanceStorageConfigAssociationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceId, resourceType, associationId, err := instanceStorageConfigParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	instanceStorageConfig, err := FindInstanceStorageAssociationByTypeWithContext(ctx, conn, instanceId, resourceType)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Instance Storage Config Association (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading Instance Storage Config Association (%s): %w", d.Id(), err))
	}

	if instanceStorageConfig == nil {
		return diag.FromErr(fmt.Errorf("error reading Instance Storage Config Association (%s): empty output", d.Id()))
	}
  d.SetId(fmt.Sprintf("%s:%s:%s", instanceId, resourceType, associationId))
	d.Set("instance_id", aws.String(d.Get("instance_id").(string)))
	d.Set("resource_type", aws.String(d.Get("resource_type").(string)))
	d.Set("storage_config", flattenInstanceStorageConfig(instanceStorageConfig))
	d.Set("association_id", associationId)

	return nil
}

func resourceInstanceStorageConfigAssociationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceId, resourceType, associationId, err := instanceStorageConfigParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	input := &connect.DisassociateInstanceStorageConfigInput{
		InstanceId:         aws.String(instanceId),
		ResourceType:       aws.String(resourceType),
		AssociationId:      aws.String(associationId),
	}

	_, err = conn.DisassociateInstanceStorageConfigWithContext(ctx, input)

	if tfawserr.ErrCodeEquals(err, connect.ErrCodeResourceNotFoundException) {
		return nil
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting Instance Storage Config Association (%s): %w", d.Id(), err))
	}

	return nil
}


func flattenInstanceStorageConfig(ec *connect.InstanceStorageConfig) []interface{} {
	if ec == nil {
		return []interface{}{}
	}
  
	storageType := aws.StringValue(ec.StorageType)

	config := map[string]interface{}{
		"storage_type": storageType,
	}
	switch storageType {
	case connect.StorageTypeS3:
		config["s3_config"] = ec.S3Config

	case connect.StorageTypeKinesisVideoStream:
		config["kinesis_video_stream_config"] = ec.KinesisVideoStreamConfig

	case connect.StorageTypeKinesisStream:
		config["kinesis_stream_config"] = ec.KinesisStreamConfig

	//case connect.StorageTypeKinesisFirehose:
	//	config.["s3_config"]: aws.struct(ec.S3Config)

	default:
		log.Printf("[ERR] storage configuration is invalid")
		return nil
	}

	return []interface{}{
		config,
	}
}

func expandInstanceStorageConfig(instanceStorageConfig []interface{}) *connect.InstanceStorageConfig {
	if len(instanceStorageConfig) == 0 || instanceStorageConfig[0] == nil {
		return nil
	}

	tfMap, ok := instanceStorageConfig[0].(map[string]interface{})
	if !ok {
		return nil
	}

	storageType := tfMap["storage_type"].(string)
	result := &connect.InstanceStorageConfig{
		StorageType: aws.String(storageType),
	}

	switch storageType {
	case connect.StorageTypeS3:
		s3cfg := tfMap["s3_config"].([]interface{})
		if len(s3cfg) == 0 || s3cfg[0] == nil {
			log.Printf("[ERR] 's3_config' must be set when 'storage_type' is '%s'", storageType)
			return nil
		}
		s3c := s3cfg[0].(map[string]interface{})
		c := connect.S3Config{
			BucketName:       aws.String(s3c["bucket_name"].(string)),
			BucketPrefix:     aws.String(s3c["bucket_prefix"].(string)),
			EncryptionConfig: expandInstanceStorageConfigEncryptionConfig(s3c["encryption_config"].([]interface{})),
		}
		result.S3Config = &c

	case connect.StorageTypeKinesisVideoStream:
		kvsc := tfMap["kinesis_video_stream_config"].([]interface{})
		if len(kvsc) == 0 || kvsc[0] == nil {
			log.Printf("[ERR] 'kinesis_video_stream_config' must be set when 'storage_type' is '%s'", storageType)
			return nil
		}
		vsc := kvsc[0].(map[string]interface{})
		sc := connect.KinesisVideoStreamConfig{
			RetentionPeriodHours:     aws.Int64(vsc["retention_period_hours"].(int64)),
			Prefix:                   aws.String(vsc["prefix"].(string)),
			EncryptionConfig:         expandInstanceStorageConfigEncryptionConfig(vsc["encryption_config"].([]interface{})),
		}
		result.KinesisVideoStreamConfig = &sc

	case connect.StorageTypeKinesisStream:
		kscfg := tfMap["kinesis_stream_config"].([]interface{})
		if len(kscfg) == 0 || kscfg[0] == nil {
			log.Printf("[ERR] 'kinesis_stream_config' must be set when 'storage_type' is '%s'", storageType)
			return nil
		}
		ksc := kscfg[0].(map[string]interface{})
		sc := connect.KinesisStreamConfig{
			StreamArn:    aws.String(ksc["stream_arn"].(string)),
		}
		result.KinesisStreamConfig = &sc

	case connect.StorageTypeKinesisFirehose:
		kfcfg := tfMap["kinesis_firehose_config"].([]interface{})
		if len(kfcfg) == 0 || kfcfg[0] == nil {
			log.Printf("[ERR] 'kinesis_firehose_config' must be set when 'storage_type' is '%s'", storageType)
			return nil
		}
		kfc := kfcfg[0].(map[string]interface{})
		fc := connect.KinesisFirehoseConfig{
			FirehoseArn:    aws.String(kfc["firehose_arn"].(string)),
		}
		result.KinesisFirehoseConfig = &fc

	default:
		log.Printf("[ERR] storage configuration is invalid")
		return nil
	}

	return result
}

func expandInstanceStorageConfigEncryptionConfig(data []interface{}) *connect.EncryptionConfig {
	if len(data) == 0 || data[0] == nil {
		return nil
	}

	ec := data[0].(map[string]interface{})
	config := &connect.EncryptionConfig{
		EncryptionType: aws.String(ec["encryption_type"].(string)),
	}
	if v, ok := ec["key_id"]; ok {
		if s := v.(string); s != "" {
			config.KeyId = aws.String(v.(string))
		}
	}
	return config
}

func instanceStorageConfigParseResourceID(id string) (string, string, string, error) {
	parts := strings.SplitN(id, ":", 3)

	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("unexpected format of Instance Storage Config Association ID (%s), expected instanceId:resourceType:associationId", id)
	}

	return parts[0], parts[1], parts[2], nil
}
