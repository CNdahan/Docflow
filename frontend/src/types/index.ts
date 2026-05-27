export type UserRole = 'super' | 'dept' | 'user';

export interface User {
  id: number;
  username: string;
  role: UserRole;
  department_id?: number | null;
  real_name: string;
  disabled: boolean;
  created_at?: string;
}

export interface Department {
  id: number;
  name: string;
  user_count?: number;
  disabled: boolean;
  created_at?: string;
}

export interface Attachment {
  id: number;
  owner_type: string;
  owner_id: number;
  purpose?: string;
  file_name: string;
  mime_type: string;
  size_bytes: number;
  uploader_id: number;
  uploaded_at: string;
}

export type TargetScope = 'DEPARTMENT' | 'ALL_USERS' | 'OWN_DEPARTMENT';

export interface DocumentItem {
  id: number;
  title: string;
  content_html: string;
  publisher_id: number;
  publisher_dept?: number | null;
  target_scope: TargetScope;
  deadline?: string | null;
  status: 'ACTIVE' | 'RECALLED';
  created_at: string;
  publisher?: User;
  stats?: DocSummary;
}

export interface DocSummary {
  total: number;
  submitted: number;
  late: number;
  pending: number;
  overdue: number;
  returned: number;
}

export type SubmissionStatus =
  | 'PENDING'
  | 'SUBMITTED'
  | 'SUBMITTED_LATE'
  | 'RETURNED'
  | 'OVERDUE';

export interface Submission {
  id: number;
  document_id: number;
  user_id: number;
  department_id?: number | null;
  current_status: SubmissionStatus;
  submitted_at?: string | null;
  return_reason: string;
  return_count: number;
  note: string;
  last_action_at: string;
  display_status?: SubmissionStatus;
  attachments?: Attachment[];
}

export interface DocumentDetail extends DocumentItem {
  reading_attachments: Attachment[];
  template_attachments: Attachment[];
  my_submission?: Submission;
  target_department_ids?: number[];
}

export interface UserSubmissionRow {
  user_id: number;
  username: string;
  real_name: string;
  department_id?: number | null;
  department_name: string;
  submission_id: number;
  current_status: SubmissionStatus;
  display_status: SubmissionStatus;
  submitted_at?: string | null;
  return_count: number;
  return_reason: string;
}

export interface DocumentOverview {
  document: DocumentItem;
  summary: DocSummary;
  by_user: UserSubmissionRow[];
  by_department?: Array<{
    department_id: number;
    department_name: string;
    total: number;
    submitted: number;
    late: number;
    pending: number;
    overdue: number;
    returned: number;
  }>;
}
