import fs from 'node:fs/promises';
import path from 'node:path';
import { Presentation, PresentationFile } from '@oai/artifact-tool';

const workspaceDir = path.resolve('D:/smetchic/cnc-estimator');
const buildDir = path.join(workspaceDir, 'promo', 'build');
const outputDir = path.join(workspaceDir, 'promo', 'output');
const draftPath = path.join(buildDir, 'cnc-estimator-storyboard-draft.pptx');
const finalPath = path.join(outputDir, 'CNC-Estimator-storyboard-v2.pptx');
await fs.mkdir(buildDir, { recursive: true });
await fs.mkdir(outputDir, { recursive: true });

const C = { bg:'#091318', panel:'#122126', panel2:'#17282a', line:'#30443e', text:'#edf4ef', muted:'#a4b8ac', lime:'#b6f36b', mint:'#68dcb3', gold:'#f1c76a', blue:'#73c6d9' };
const p = Presentation.create({ slideSize:{ width:1280, height:720 } });

function rect(slide,x,y,w,h,fill,r=0,stroke='none',sw=0){
  const sh=slide.shapes.add({geometry:r?'roundRect':'rect',position:{left:x,top:y,width:w,height:h},fill,line:{style:'solid',fill:stroke,width:sw},...(r?{borderRadius:'rounded-lg'}:{})});return sh;
}
function ellipse(slide,x,y,w,h,fill,stroke='none',sw=0){return slide.shapes.add({geometry:'ellipse',position:{left:x,top:y,width:w,height:h},fill,line:{style:'solid',fill:stroke,width:sw}})}
function line(slide,x1,y1,x2,y2,color=C.line,sw=2){const dx=x2-x1,dy=y2-y1;return slide.shapes.add({geometry:'line',position:{left:Math.min(x1,x2),top:Math.min(y1,y2),width:Math.abs(dx),height:Math.abs(dy),verticalFlip:dx*dy<0},fill:'none',line:{style:'solid',fill:color,width:sw}})}
function text(slide,t,x,y,w,h,size,color=C.text,bold=false,opts={}){
 const sh=slide.shapes.add({geometry:'textbox',position:{left:x,top:y,width:w,height:h},fill:'none',line:{style:'solid',fill:'none',width:0}});sh.text=t;sh.text.style={typeface:'Segoe UI',fontSize:size,color,bold,wrap:true,...opts};return sh;
}
function base(kicker,slideNo){
 const s=p.slides.add();s.background.fill=C.bg;
 rect(s,0,0,10,720,C.lime);text(s,'SMETCHIC  /  CNC ESTIMATOR',64,36,360,24,13,C.muted,true);text(s,kicker.toUpperCase(),64,106,640,24,14,C.lime,true);
 text(s,String(slideNo).padStart(2,'0'),1180,38,40,24,13,C.muted,true);
 line(s,64,650,1216,650,'#20332f',1);text(s,'MVP · ПРЕДВАРИТЕЛЬНАЯ ОЦЕНКА · НЕ CAM-РАСЧЁТ',64,671,600,18,11,'#7f978a',true);
 return s;
}
function chip(s,label,x,y,w,color=C.lime){rect(s,x,y,w,32,'#1a3026',8);text(s,label,x+12,y+7,w-24,18,11,color,true)}
function uiFrame(s,x,y,w,h,title){rect(s,x,y,w,h,C.panel,14,C.line,1);rect(s,x,y,w,46,'#19292b',14);rect(s,x,y+28,w,18,'#19292b');ellipse(s,x+18,y+18,9,9,'#e97567');ellipse(s,x+34,y+18,9,9,'#e7bd62');ellipse(s,x+50,y+18,9,9,'#79c88a');text(s,title,x+74,y+13,w-100,22,13,'#dce8df',true)}

// 1 — project and positioning
{
 const s=p.slides.add();s.background.fill=C.bg;rect(s,0,0,10,720,C.lime);
 text(s,'SMETCHIC  /  CNC ESTIMATOR',72,46,450,26,14,C.muted,true);
 chip(s,'ПРОТОТИП ИЩЕТ ПАРТНЁРА ДЛЯ ПИЛОТА',72,139,350);
 text(s,'От детали — к\nпредварительной смете',72,211,710,154,48,C.text,true);
 text(s,'Веб-инструмент для оценки затрат на изготовление\nдеталей на станках с ЧПУ.',76,390,600,74,21,C.muted);
 // restrained isometric workpiece illustration
 const cx=1000,cy=340;
 line(s,cx-150,cy+110,cx,cy+24,C.lime,4);line(s,cx,cy+24,cx+154,cy+112,C.lime,4);line(s,cx+154,cy+112,cx+2,cy+198,C.lime,4);line(s,cx+2,cy+198,cx-150,cy+110,C.lime,4);
 line(s,cx-150,cy+110,cx-150,cy+194,C.lime,4);line(s,cx-150,cy+194,cx+2,cy+280,C.lime,4);line(s,cx+2,cy+280,cx+2,cy+198,C.lime,4);line(s,cx+2,cy+280,cx+154,cy+194,C.lime,4);line(s,cx+154,cy+194,cx+154,cy+112,C.lime,4);
 ellipse(s,cx-35,cy+77,70,38,C.bg,C.mint,3);ellipse(s,cx-18,cy+86,36,20,'#18332c',C.mint,2);
 chip(s,'ФАЙЛ ДЕТАЛИ',72,544,147,C.mint);chip(s,'СТАНОК',232,544,102,C.blue);chip(s,'ЭКОНОМИКА',347,544,130,C.gold);
}
// 2 — source file and estimate entry
{
 const s=base('Рабочий процесс · 01',2);text(s,'Загрузка детали',64,154,490,55,38,C.text,true);text(s,'Начните с файла .part и основных параметров заказа.',66,216,500,58,18,C.muted);
 uiFrame(s,616,144,600,448,'Новая смета');
 text(s,'Файл детали',644,214,240,22,13,C.muted,true);rect(s,644,244,544,112,'#14262b',9,C.line,1);
 // file icon
 rect(s,673,270,40,54,'none',4,C.lime,2);line(s,700,270,713,283,C.lime,2);line(s,700,270,700,284,C.lime,2);line(s,700,284,713,284,C.lime,2);
 text(s,'bracket_v07.part',737,273,330,26,17,C.text,true);text(s,'Исходник сохраняется вместе с версией детали',737,306,400,22,12,C.muted);
 text(s,'Материал',644,393,190,20,12,C.muted);rect(s,644,407,250,48,'#19292b',7,C.line,1);text(s,'Алюминий 6061',660,419,210,24,15,C.text);
 text(s,'Количество',922,393,190,20,12,C.muted);rect(s,922,407,266,48,'#19292b',7,C.line,1);text(s,'20 шт.',938,419,225,24,15,C.text);
 rect(s,644,496,176,46,C.lime,8);text(s,'Рассчитать',669,507,135,26,15,'#122017',true);
 text(s,'Загрузка и расчёт показаны как демонстрационный сценарий.',66,550,490,42,12,'#82998c');
}
// 3 — company manufacturing context
{
 const s=base('Рабочий процесс · 02',3);text(s,'Параметры производства',64,154,580,55,36,C.text,true);text(s,'Условия компании влияют на итоговую оценку.',66,216,510,36,18,C.muted);
 const xs=[64,454,844], labels=[['ОБОРУДОВАНИЕ','Станок и ставка','Модель\nПочасовая ставка'],['МАТЕРИАЛЫ','Материал и заготовка','Тип материала\nРазмер и цена'],['РАСХОДЫ','Экономика компании','Персонал\nИнструмент и накладные']];
 labels.forEach((a,i)=>{let x=xs[i];rect(s,x,309,346,236,C.panel,13,C.line,1);ellipse(s,x+26,338,52,52,['#244b39','#244251','#4a4228'][i]);text(s,String(i+1).padStart(2,'0'),x+29,352,46,23,14,[C.lime,C.blue,C.gold][i],true);text(s,a[0],x+26,416,290,18,12,C.muted,true);text(s,a[1],x+26,442,292,28,19,C.text,true);text(s,a[2],x+26,486,280,46,14,C.muted)});
 chip(s,'НАСТРОЙКИ ХРАНЯТСЯ В КОНТЕКСТЕ КОМПАНИИ',64,575,392,C.lime);
}
// 4 — cost estimate, with caveat
{
 const s=base('Рабочий процесс · 03',4);text(s,'Структура расчёта',64,148,780,55,36,C.text,true);text(s,'Версия сметы сохраняет исходные данные и детализацию.',66,211,740,34,18,C.muted);
 uiFrame(s,66,284,700,304,'Результат расчёта');
 const rows=[['Станочное время','расчёт по параметрам'],['Работа оборудования','ставка компании'],['Материал и заготовка','цены каталога'],['Прочие расходы','только при настройке']];
 rows.forEach((r,i)=>{let y=353+i*48;text(s,r[0],94,y,300,23,14,C.muted);text(s,r[1],388,y,332,23,14,C.text,true);line(s,94,y+29,736,y+29,'#263b35',1)});
 rect(s,94,533,642,38,'#1b3027',7);text(s,'Отсутствующие расходы не подставляются автоматически',109,542,610,20,12,C.lime,true);
 rect(s,804,284,410,304,'#171f1e',13,'#5e4c2c',1);text(s,'Предварительная оценка',832,316,350,28,18,C.gold,true);text(s,'Результат зависит от\nвведённых исходных данных\nи настроек компании.',832,365,345,106,19,C.text);text(s,'Не заменяет CAM-расчёт и\nкоммерческое предложение.',832,492,345,62,15,C.muted);
}
// 5 — projects and estimate versions
{
 const s=base('Рабочее пространство · 04',5);text(s,'Проекты и версии смет',64,154,610,55,38,C.text,true);text(s,'Расчёты связаны с деталями, заказами и историей изменений.',66,216,800,36,18,C.muted);
 uiFrame(s,216,300,850,274,'Проект · Кронштейн крепления');
 text(s,'ДЕТАЛЬ / ВЕРСИЯ',242,365,280,18,12,C.muted,true);text(s,'ИЗМЕНЕНИЕ',555,365,280,18,12,C.muted,true);text(s,'СТАТУС',906,365,120,18,12,C.muted,true);
 const versions=[['03 · Текущая','Обновлены настройки расчёта','АКТУАЛЬНАЯ'],['02','Изменена заготовка','СОХРАНЕНА'],['01','Первичный расчёт','СОХРАНЕНА']];
 versions.forEach((r,i)=>{let y=401+i*48;if(i===0)rect(s,242,y-10,790,40,'#183027',6);text(s,r[0],254,y,245,22,14,i===0?C.lime:C.text,true);text(s,r[1],555,y,320,22,13,C.muted);text(s,r[2],906,y,130,22,10,i===0?C.lime:C.mint,true);if(i<2)line(s,242,y+30,1032,y+30,'#263b35',1)});
 chip(s,'ДАННЫЕ РАСЧЁТА СОХРАНЯЮТСЯ В ВЕРСИИ',216,603,338,C.mint);
}
// 6 — honest partner invitation
{
 const s=p.slides.add();s.background.fill=C.bg;rect(s,0,0,10,720,C.lime);text(s,'SMETCHIC  /  CNC ESTIMATOR',72,46,450,26,14,C.muted,true);
 chip(s,'СЛЕДУЮЩИЙ ЭТАП — ВМЕСТЕ',72,145,235);
 text(s,'Ищем компанию\nдля пилотного\nсотрудничества',72,211,665,222,43,C.text,true);
 text(s,'Покажем MVP, проверим рабочий сценарий\nна реальной задаче и согласуем приоритеты\nследующего этапа.',76,462,600,100,18,C.muted);
 rect(s,800,183,416,352,C.panel,16,C.line,1);text(s,'ОБСУДИМ',832,221,320,22,13,C.lime,true);
 const items=['Демонстрацию прототипа','Пилот на задаче компании','Финансирование доработки'];items.forEach((a,i)=>{ellipse(s,834,277+i*69,26,26,'#233a2d');text(s,'✓',840,279+i*69,18,20,14,C.lime,true);text(s,a,878,280+i*69,300,22,15,C.text,true)});
 line(s,832,481,1184,481,'#30443e',1);text(s,'CNC Estimator · MVP',832,497,320,21,14,C.muted,true);
 text(s,'ОТКРЫТ К СОТРУДНИЧЕСТВУ И ФИНАНСИРОВАНИЮ',72,632,800,24,12,C.lime,true);text(s,'Контакты: добавьте имя · телефон / Telegram · ссылку на демо',72,671,900,20,12,'#82998c');
}

for(const [i,s] of p.slides.items.entries()){
  s.speakerNotes.text='Кадр '+(i+1)+' из проморолика CNC Estimator. Описание основано на текущем MVP и его README. UI в кадре — стилизованная иллюстрация функций, не запись работающего интерфейса.';
}

const { finalizePresentation } = await import('file:///C:/Users/Kalkulator/.codex/plugins/cache/openai-primary-runtime/presentations/26.1112.11217/skills/presentations/container_tools/artifact_tool_utils.mjs');
const expectedSlideSizeEmu='12192000,6858000';
const candidatePath=path.join(buildDir,'candidate.pptx');
const pptx=await PresentationFile.exportPptx(p);await pptx.save(candidatePath);
const result=await finalizePresentation({workspaceDir,candidatePath,finalPath,pythonExecutable:'C:/Users/Kalkulator/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe',integrityValidatorPath:'C:/Users/Kalkulator/.codex/plugins/cache/openai-primary-runtime/presentations/26.1112.11217/skills/presentations/container_tools/inspect_presentation_package_integrity.py',layoutValidatorPath:'C:/Users/Kalkulator/.codex/plugins/cache/openai-primary-runtime/presentations/26.1112.11217/skills/presentations/container_tools/inspect_presentation_layout_geometry.py',layoutArgs:['--expected-slide-size-emu',expectedSlideSizeEmu,'--validate-bullet-geometry','--validate-heading-fit'],requiredNativeTableOwnerSlides:[],fontPolicy:{basis:'design',families:['Segoe UI']},verifyArtifactToolImport:true,receiptPath:path.join(buildDir,'validation-v2.json')});
console.log(JSON.stringify(result));
for(const [i,s] of p.slides.items.entries()){
 const blob=await p.export({slide:s,format:'png',scale:1});await fs.writeFile(path.join(buildDir,`slide-${String(i+1).padStart(2,'0')}.png`),new Uint8Array(await blob.arrayBuffer()));
}
const montage=await p.export({format:'png',montage:true});await fs.writeFile(path.join(buildDir,'montage.png'),new Uint8Array(await montage.arrayBuffer()));
