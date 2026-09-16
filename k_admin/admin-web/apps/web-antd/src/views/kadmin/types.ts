export type ResourceStatus = 'online' | 'pending' | 'offline';
export type ResourcePriority = 'high' | 'medium' | 'low';

export interface ResourceItem {
  key: string;
  name: string;
  type: string;
  owner: string;
  status: ResourceStatus;
  priority: ResourcePriority;
  progress: number;
  enabled: boolean;
  createdAt: string;
  tags: string[];
}
