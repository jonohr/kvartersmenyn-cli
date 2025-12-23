# Contributing

This repo uses a simple feature-branch workflow.

## Workflow

```bash
git checkout -b feature/<short-name>
# make changes
git add .
git commit -m "Describe the change"
git push -u origin feature/<short-name>
```

Open a PR from your branch into `main`. When merged, tag a release if it should ship.

## Release

```bash
git tag vX.Y.Z
git push --tags
```
