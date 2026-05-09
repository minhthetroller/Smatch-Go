export interface User {
  id: string;
  firebaseUid: string;
  email?: string;
  username?: string;
  provider: string;
  isAnonymous: boolean;
  firstName?: string;
  lastName?: string;
  gender?: string;
  phoneNumber?: string;
  photoUrl?: string;
  address?: {
    street?: string;
    ward?: string;
    district?: string;
    city?: string;
  };
  roles: string[];
  createdAt: string;
  updatedAt: string;
}

export interface BusinessProfileResponse {
  id: string;
  userId: string;
  legalRepresentativeName: string;
  personalIdNumber: string;
  personalIdFrontImageUrl?: string;
  personalIdBackImageUrl?: string;
  businessRegistrationCertUrl?: string;
  sportsBusinessEligibilityCertUrl?: string;
  fireSafetyCertUrl?: string;
  taxIdNumber: string;
  proofOfAddressUrl?: string;
  bankAccountNumber: string;
  bankName: string;
  bankBranch: string;
  bankAccountHolderName: string;
  operationalSpecs: OperationalSpecs;
  status: 'pending' | 'approved' | 'rejected' | 'resubmit_requested';
  adminNotes?: string;
  submittedAt: string;
  reviewedAt?: string;
  reviewedBy?: string;
}

export interface OperationalSpecs {
  subcourt_count: number;
  surface_type: string;
  operating_hours: {
    open: string;
    close: string;
  };
  base_pricing: BasePricingRule[];
}

export interface BasePricingRule {
  day_type: string;
  start_time: string;
  end_time: string;
  price_per_hour: number;
}

export interface SubmitBusinessProfileRequest {
  legalRepresentativeName: string;
  personalIdNumber: string;
  taxIdNumber: string;
  bankAccountNumber: string;
  bankName: string;
  bankBranch: string;
  bankAccountHolderName: string;
  operationalSpecs: OperationalSpecs;
}

export interface ReviewBusinessProfileRequest {
  action: 'approve' | 'reject' | 'request_resubmit';
  adminNotes: string;
}

export interface CourtOwnerCourtResponse {
  id: string;
  name: string;
  description?: string;
  phoneNumbers: string[];
  addressStreet?: string;
  addressWard?: string;
  addressDistrict?: string;
  addressCity?: string;
  details: unknown;
  openingHours: unknown;
  lat?: number;
  lng?: number;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CourtStatsResponse {
  totalBookings: number;
  totalRevenue: number;
  occupancyRate: number;
  cancellationRate: number;
}

export interface SubCourtResponse {
  id: string;
  name: string;
  description?: string;
  isActive: boolean;
}

export interface PricingRuleResponse {
  id: string;
  name: string;
  dayType: string;
  startTime: string;
  endTime: string;
  pricePerHour: number;
  isActive: boolean;
}

export interface SubCourtClosureResponse {
  id: string;
  subCourtId: string;
  date: string;
  startTime?: string;
  endTime?: string;
  reason?: string;
}

export interface CourtOwnerCourtDetailResponse {
  id: string;
  name: string;
  description?: string;
  phoneNumbers: string[];
  addressStreet?: string;
  addressWard?: string;
  addressDistrict?: string;
  addressCity?: string;
  details: unknown;
  openingHours: unknown;
  lat?: number;
  lng?: number;
  isActive: boolean;
  subCourts: SubCourtResponse[];
  pricingRules: PricingRuleResponse[];
  upcomingClosures: SubCourtClosureResponse[];
  createdAt: string;
  updatedAt: string;
}

export interface CourtStatsDailyItem {
  date: string;
  bookings: number;
  revenue: number;
  cancellations: number;
}

export interface CourtStatsDetailResponse {
  summary: CourtStatsResponse;
  dailyBreakdown?: CourtStatsDailyItem[];
}

export interface CloseCourtRequest {
  date: string;
  startTime?: string;
  endTime?: string;
  reason: string;
}

export interface CloseSubCourtRequest {
  date: string;
  startTime?: string;
  endTime?: string;
  reason: string;
}

export interface AdminStatsResponse {
  totalActiveUsers: number;
  totalCourtOwners: number;
  totalCourts: number;
  totalRevenue: number;
  pendingApplications: number;
  recentSignups: number;
}

export interface TimeseriesPoint {
  label: string;
  value: number;
}

export interface AdminTimeseriesResponse {
  range: string;
  signups: TimeseriesPoint[];
  activeUsers: TimeseriesPoint[];
  revenue: TimeseriesPoint[];
}

export interface PaginationMeta {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
}

export interface PaginatedResponse<T> {
  success: boolean;
  data: T[];
  meta: {
    pagination: PaginationMeta;
  };
}

export interface ApiError {
  success: false;
  error: {
    message: string;
    code: string;
  };
}
