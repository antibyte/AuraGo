# Bounded PDF text extraction

## Purpose

Shared pure-Go PDF parsing for the indexer, PDF tool and PDF export verification.

## Ownership

- `Extract` owns request cancellation, file/page/text limits and complete results.
- `parser/` retains the BSD-3-Clause ledongthuc/pdf sources at
  `6c8c28e0e8a0`, with local parsing, allocation and decompression guards.
- `pdftest/` builds synthetic PDFs for the boundary and cross-component tests.
- Services own index publication and media permission; tools own rooted input
  access and external-data isolation. This package never publishes or calls a provider.

## Local Contracts

- Cap input at 32 MiB, pages at 1,000, each decoded stream at 16 MiB,
  aggregate decoding at 128 MiB, text at 8 MiB and parsing at 30 seconds.
- Count each filter's decoded bytes once, before predictor processing. Validate
  predictor columns, default omitted columns to one and retain no-prediction mode.
  Recheck shared byte allowances and cancellation after nested decoder reads;
  exhaustion must retain `ErrLimit`, never become a generic parse error.
  Empty filter arrays use the same bound as unfiltered streams.
- Context checks reach tokenization, indirect references and page traversal.
  Check document-controlled sizes before allocation. Limit and cancellation
  errors are terminal; never publish partial text or send a fallback request.
- Preserve upstream copyright and the verbatim `parser/LICENSE`. Keep the
  upstream pin and local patch description in `documentation/dependencies.md`.

## Work Guidance

- Keep parser changes limited to bounds, cancellation and error propagation.
  Refresh upstream only with the malicious-input and export regressions passing.
- Use synthetic small PDF fixtures; never copy private documents into tests.

## Verification

- `go test ./internal/pdftext/...` plus PDF tests in services, tools, Detective
  and Infra; index replacement tests verify failure preserves the old generation.
- `TestExtractDecoderLimits` covers predictor and chained-filter exhaustion;
  `TestLimitsReachTokenizerAndAggregateStreams` covers exact accounting and aborts.

## Child DOX Index

None.
