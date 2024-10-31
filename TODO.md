### Storage
- Create Storage class:
  - Encryption enabled (KMS)
  - Reclaim policy: Retain
  - WaitForFirstCnsumer to ensure volume is created in the same zone where pod is going to run
  - Allow Volume expansion
  - Specify allowed zones
  - Also configure statefulsets with EBS retention
    - WhenDeleted: Retain
    - WhenScaled: Retain
#### Backups
  - Configure automated backups
  - Create CRD to enable restore snapshots

### Topology 
  - Configure node affinity to ensure pods are scheduled in the same zone as EBS volumes.
  - Topology constraints
    - Node selector
    - Multi AZ

### Upgrades
  - Auto-upgrades with ReplicasFirstPrimaryLast config

## Limitations

- Adding more replicas require restarting the existing cluster entirely