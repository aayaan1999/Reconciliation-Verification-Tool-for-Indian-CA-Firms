// The sample data the screens show until they are connected to the engine (SPEC-004).
// run-*.json and facts-*.json are the engine's own output on testdata/run, and worklist.json
// its Worklist on Test Pack A's two ledgers, kept equal to it by TestWebSamplesMatchEngine
// in cmd/recon-engine. clients.json is hand-made. Every name, PAN and figure is invented.
import type { Client, Comparison, FactList, RuleList, RunReport, WorklistItem } from '../types.ts'
import clientsFile from './clients.json'
import comparisonsFile from './comparisons.json'
import factsClean from './facts-clean.json'
import factsCompare from './facts-compare-differs.json'
import factsDiffers from './facts-differs.json'
import factsGstPurchase from './facts-gst-purchase.json'
import factsFilingPack from './facts-filing-pack.json'
import factsGstSales from './facts-gst-sales.json'
import factsMissing from './facts-missing.json'
import runClean from './run-clean.json'
import runCompare from './run-compare-differs.json'
import runDiffers from './run-differs.json'
import runFilingPack from './run-filing-pack.json'
import runGstPurchase from './run-gst-purchase.json'
import runGstSales from './run-gst-sales.json'
import rulesFile from './rules.json'
import runMissing from './run-missing.json'
import worklistFile from './worklist.json'

export interface SampleRun {
  report: RunReport
  facts: FactList
  last_run_on?: string // a client's stored result: the date of that type's last run
}

export const sampleRuns: Record<string, SampleRun> = {
  clean: { report: runClean as RunReport, facts: factsClean as FactList },
  differs: { report: runDiffers as RunReport, facts: factsDiffers as FactList },
  missing: { report: runMissing as RunReport, facts: factsMissing as FactList },
  'compare-differs': { report: runCompare as RunReport, facts: factsCompare as FactList },
  'gst-sales': { report: runGstSales as RunReport, facts: factsGstSales as FactList },
  'gst-purchase': { report: runGstPurchase as RunReport, facts: factsGstPurchase as FactList },
  'filing-pack': { report: runFilingPack as RunReport, facts: factsFilingPack as FactList },
}

export const sampleTaxYear: string = clientsFile.tax_year
export const sampleClients: Client[] = clientsFile.clients
export const sampleWorklist: WorklistItem[] = worklistFile.items as WorklistItem[]
export const sampleRules: RuleList = rulesFile as RuleList
export const sampleComparisons: Comparison[] = comparisonsFile.comparisons as Comparison[]

// The raw files, for the shape test.
export const sampleFiles: Record<string, unknown> = {
  'clients.json': clientsFile,
  'worklist.json': worklistFile,
  'rules.json': rulesFile,
  'comparisons.json': comparisonsFile,
  'run-clean.json': runClean,
  'run-differs.json': runDiffers,
  'run-missing.json': runMissing,
  'facts-clean.json': factsClean,
  'facts-differs.json': factsDiffers,
  'facts-missing.json': factsMissing,
  'run-compare-differs.json': runCompare,
  'facts-compare-differs.json': factsCompare,
  'run-gst-sales.json': runGstSales,
  'facts-gst-sales.json': factsGstSales,
  'run-gst-purchase.json': runGstPurchase,
  'facts-gst-purchase.json': factsGstPurchase,
  'run-filing-pack.json': runFilingPack,
  'facts-filing-pack.json': factsFilingPack,
}
