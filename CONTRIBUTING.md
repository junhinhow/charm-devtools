# Contributing

Thank you for your interest in contributing!

## How to contribute

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes following [Conventional Commits](https://www.conventionalcommits.org/)
4. Run tests: `go test ./...`
5. Commit with a descriptive message: `feat(scope): add feature X`
6. Push and open a Pull Request

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat(scope):` — new feature
- `fix(scope):` — bug fix
- `docs:` — documentation only
- `test:` — adding tests
- `chore:` — maintenance

## Pull Request Guidelines

- One logical change per PR
- Include tests for new features
- Update documentation if needed
- Reference related issues with `Closes #N`

## Development

```bash
go build ./...
go test ./...
go vet ./...
```
