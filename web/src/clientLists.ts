import type { ClientDetails } from './types.ts'

// The kinds of client, GST registration types and GST state codes. They mirror
// internal/client (a Go test keeps them equal).
export const constitutions: [string, string][] = [
  ['company', 'Company'], ['llp', 'LLP'], ['partnership_firm', 'Partnership firm'], ['proprietorship', 'Proprietorship'],
  ['individual', 'Individual'], ['huf', 'HUF'], ['trust', 'Trust'], ['aop_boi', 'AOP / BOI'], ['other', 'Other'],
]
export const registrationTypes: [string, string][] = [
  ['regular', 'Regular'], ['composition', 'Composition'], ['sez_unit', 'SEZ unit'], ['sez_developer', 'SEZ developer'],
  ['isd', 'Input service distributor'], ['tds_tcs', 'TDS / TCS'], ['non_resident', 'Non-resident'], ['casual', 'Casual'],
]
export const states: [string, string][] = [
  ['01', 'Jammu and Kashmir'], ['02', 'Himachal Pradesh'], ['03', 'Punjab'], ['04', 'Chandigarh'], ['05', 'Uttarakhand'],
  ['06', 'Haryana'], ['07', 'Delhi'], ['08', 'Rajasthan'], ['09', 'Uttar Pradesh'], ['10', 'Bihar'], ['11', 'Sikkim'],
  ['12', 'Arunachal Pradesh'], ['13', 'Nagaland'], ['14', 'Manipur'], ['15', 'Mizoram'], ['16', 'Tripura'], ['17', 'Meghalaya'],
  ['18', 'Assam'], ['19', 'West Bengal'], ['20', 'Jharkhand'], ['21', 'Odisha'], ['22', 'Chhattisgarh'], ['23', 'Madhya Pradesh'],
  ['24', 'Gujarat'], ['26', 'Dadra and Nagar Haveli and Daman and Diu'], ['27', 'Maharashtra'], ['29', 'Karnataka'], ['30', 'Goa'],
  ['31', 'Lakshadweep'], ['32', 'Kerala'], ['33', 'Tamil Nadu'], ['34', 'Puducherry'], ['35', 'Andaman and Nicobar Islands'],
  ['36', 'Telangana'], ['37', 'Andhra Pradesh'], ['38', 'Ladakh'], ['97', 'Other Territory'],
]

export function emptyDetails(): ClientDetails {
  return {
    legal_name: '', trade_name: '', pan: '', pan_last4: '', constitution: '', date_of_incorporation: '', cin: '', tan: '',
    gstins: [], address: { line1: '', line2: '', city: '', state: '', pin: '' },
    contact_person: '', email: '', phone: '', accounting_software: '', notes: '',
  }
}
