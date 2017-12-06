#!/bin/bash

set -eu -o pipefail

director_name="$(bosh-cli int "${ENV_FILE}" --path=/director_name)"
director_ip="$(bosh-cli int "${ENV_FILE}" --path=/internal_ip)"
subnet_id="$(bosh-cli int "${ENV_FILE}" --path=/subnet_id)"
access_key_id="$(bosh-cli int "${ENV_FILE}" --path=/access_key_id)"
secret_access_key="$(bosh-cli int "${ENV_FILE}" --path=/secret_access_key)"
region="$(bosh-cli int "${ENV_FILE}" --path=/region)"

mkdir -p ~/.aws

cat > ~/.aws/credentials <<-EOF
[default]
aws_access_key_id=${access_key_id}
aws_secret_access_key=${secret_access_key}
EOF

cat > ~/.aws/config <<-EOF
[default]
region=${region}
output=text
EOF

delete_volumes() {
  local volume_ids=$1
  for volume in $volume_ids; do
    aws ec2 delete-volume --volume-id "$volume"
    aws ec2 wait volume-deleted --volume-id "$volume"
  done
}

director_instance_id=$(aws ec2 describe-instances --query 'Reservations[*].Instances[*].InstanceId' --output text --filters "Name=network-interface.addresses.private-ip-address,Values=${director_ip}" "Name=subnet-id,Values=${subnet_id}")
if [ -z "$director_instance_id" ]; then
  echo "No instance found for BOSH Director IP address"
else
  aws ec2 terminate-instances --instance-ids "$director_instance_id"
  aws ec2 wait instance-terminated --filters "Name=instance-id,Values=${director_instance_id}"
fi

instance_ids=$(aws ec2 describe-instances --query 'Reservations[*].Instances[*].InstanceId' --filters "Name=subnet-id,Values=${subnet_id}")
if [ -z "$instance_ids" ]; then
  echo "No instances found in subnet '${subnet_id}'"
else
  aws ec2 terminate-instances --instance-ids ${instance_ids}
  aws ec2 wait instance-terminated --instance-ids ${instance_ids}
fi

director_name_tag_volume_ids=$(aws ec2 describe-volumes --output text --no-paginate --query 'Volumes[*].VolumeId' --filters "Name=tag:director,Values=${director_name}" "Name=status,Values=available")
#director_volume_ids=$(aws ec2 describe-volumes --output text --no-paginate --query 'Volumes[*].VolumeId' --filters "Name=attachment.instance-id,Values=${director_instance_id}" "Name=status,Values=available")

#if [ ! -z $director_volume_ids ]
#  echo "Deleting volumes associated to director '${director_name}'"
#  delete_volumes director_volume_ids
#fi
if [ ! -z "$director_name_tag_volume_ids" ]; then
  echo "Deleting volumes tagged with director name '${director_name}'"
  delete_volumes "$director_name_tag_volume_ids"
fi
