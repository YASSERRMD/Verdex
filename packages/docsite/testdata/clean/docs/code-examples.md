# Test fixture: fenced code blocks must never be scanned for links

Go generic function-call syntax inside a fenced block looks exactly
like a markdown inline link to a naive regex -- this file proves
CheckLinks does not misread it as one.

```go
guard := reliability.NewIdempotencyGuard[PaymentResult](10 * time.Minute)
result, err := guard.Execute(ctx, requestID, func(ctx context.Context) (PaymentResult, error) {
    return chargeCard(ctx)
})
```

A real link right after the fenced block, to prove scanning resumes
correctly once the block closes: [docs index](README.md).

~~~text
[also skipped](inside-a-tilde-fence.md)
~~~

Another real link after the tilde-fenced block: [foo package doc](../packages/foo/doc/foo.md).
