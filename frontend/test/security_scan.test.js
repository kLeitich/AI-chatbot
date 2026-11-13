import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'

describe('Frontend Source Security Scan', () => {
  const forbiddenPatterns = [
    'admin123',
    'platform123',
    'admin@example.com',
    'platform@example.com',
  ]

  const ignoredDirs = new Set(['node_modules', '.next', '.git', 'test'])

  function getAllFiles(dirPath, arrayOfFiles = []) {
    const files = fs.readdirSync(dirPath)

    for (const file of files) {
      const fullPath = path.join(dirPath, file)
      if (fs.statSync(fullPath).isDirectory()) {
        if (!ignoredDirs.has(file)) {
          getAllFiles(fullPath, arrayOfFiles)
        }
      } else {
        const ext = path.extname(file).toLowerCase()
        if (['.js', '.jsx', '.ts', '.tsx', '.json', '.html', '.css', '.md'].includes(ext)) {
          // exclude package-lock.json and vitest/eslint configs
          if (file !== 'package-lock.json') {
            arrayOfFiles.push(fullPath)
          }
        }
      }
    }

    return arrayOfFiles
  }

  it('contains no forbidden default credentials or example emails in source files', () => {
    const frontendRoot = path.resolve(__dirname, '..')
    const files = getAllFiles(frontendRoot)
    const violations = []

    for (const filePath of files) {
      const content = fs.readFileSync(filePath, 'utf-8')
      for (const pattern of forbiddenPatterns) {
        if (content.includes(pattern)) {
          violations.push({
            file: path.relative(frontendRoot, filePath),
            pattern,
          })
        }
      }
    }

    expect(violations).toEqual([])
  })
})

