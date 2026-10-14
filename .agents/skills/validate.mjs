import fs from 'node:fs';
import path from 'node:path';

const root = path.resolve('.agents/skills');
const manifestPath = path.join(root, 'manifest.json');
if (!fs.existsSync(manifestPath)) throw new Error('missing .agents/skills/manifest.json');

const dirs = fs.readdirSync(root,{withFileTypes:true})
  .filter(d=>d.isDirectory())
  .map(d=>d.name)
  .filter(n=>!n.startsWith('.'))
  .sort();

const names=[];
for (const dir of dirs) {
  const file=path.join(root,dir,'SKILL.md');
  if (!fs.existsSync(file)) throw new Error(`missing SKILL.md in ${dir}`);
  const text=fs.readFileSync(file,'utf8');
  if (!text.startsWith('---\n')) throw new Error(`${dir}: missing YAML frontmatter`);
  const end=text.indexOf('\n---\n',4);
  if (end<0) throw new Error(`${dir}: unterminated YAML frontmatter`);
  const front=text.slice(4,end);
  const field=(key)=>{
    const m=front.match(new RegExp(`^${key}:\\s*(.+)$`,'m'));
    return m?m[1].trim():'';
  };
  const name=field('name'), description=field('description'), version=field('version');
  if (!name || !description || !version) throw new Error(`${dir}: name/description/version required`);
  if (name!==dir) throw new Error(`${dir}: frontmatter name "${name}" must match directory`);
  names.push(name);
}
if (new Set(names).size!==names.length) throw new Error('duplicate skill names');

const manifest=JSON.parse(fs.readFileSync(manifestPath,'utf8'));
if (!Array.isArray(manifest.skills)) throw new Error('manifest.skills must be an array');
const listed=[...manifest.skills].sort();
if (new Set(listed).size!==listed.length) throw new Error('duplicate names in manifest.skills');
if (JSON.stringify(listed)!==JSON.stringify(names)) {
  throw new Error(`manifest mismatch: actual=${names.join(',')} listed=${listed.join(',')}`);
}
console.log(`AGENT_SKILLS_VALID: ${names.length} skills`);
