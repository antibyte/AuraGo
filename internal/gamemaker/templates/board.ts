import { GameScene, start } from './common';
// Three rounds against the machine; each round it plays smarter. Round 1 takes free
// cells, round 2 also blocks your lines, round 3 also completes its own and takes the
// centre. It replies half a second after your move; win a round to continue.
const LINES=[[0,1,2],[3,4,5],[6,7,8],[0,3,6],[1,4,7],[2,5,8],[0,4,8],[2,4,6]];
class Board extends GameScene {
  selected=0; cells:any[]=[]; marks:number[]=[]; replyAt=0;
  setup() { if (this.setupScene()) return;
    this.configureLevels([{id:'novice',title:'Round 1: the novice'},{id:'blocker',title:'Round 2: the blocker'},{id:'master',title:'Round 3: the master'}]);
    this.selected=0;this.marks=Array(9).fill(0);this.cells=[];this.replyAt=0;
    for(let i=0;i<9;i++){
      const cell=this.add.rectangle(360+(i%3)*100,180+Math.floor(i/3)*100,90,90,0x334155).setInteractive();
      this.paintAsset(cell,90,90,'cell');
      // Keep a transparent hit target when library art replaces the cell.
      if(this.assetRoles('cell').length)cell.setVisible(true).setFillStyle(0,0);
      cell.on('pointerdown',()=>{this.selected=i;this.action();});this.cells.push(cell);
    }
    this.player=this.add.rectangle(360,180,96,96).setStrokeStyle(3,0xfacc15);
  }
  place(index:number,turn:number) {
    this.marks[index]=turn;const cell=this.cells[index];
    cell.setFillStyle(turn===1?0x5eead4:0xfb7185);
    const mark=this.add.rectangle(cell.x,cell.y,60,60,turn===1?0x5eead4:0xfb7185);
    this.paintAsset(mark,60,60,this.assetRoles('marker_'+turn).length?'marker_'+turn:'marker');
    this.state.turns++;
    const won=LINES.some(line=>line.every(i=>this.marks[i]===turn));
    if(won||this.marks.every(Boolean)){this.flow.message(won?(turn===1?'You win the round!':'The machine wins.'):'Draw.',1.6);this.end(won&&turn===1);}
  }
  action() { if (this.builder) { super.action(); return; }
    if(this.marks[this.selected]||this.replyAt)return;
    this.feedback('ui',this.cells[this.selected]);this.state.actions++;this.state.hits++;this.state.score++;
    this.place(this.selected,1);
    if(!this.state.ended)this.replyAt=this.elapsed+500;
  }
  // The machine completes (round 3) or blocks (round 2+) a line, else takes a free cell.
  machineMove() {
    const lineFor=(turn:number)=>{for(const line of LINES){const empty=line.filter(i=>!this.marks[i]);if(empty.length===1&&line.filter(i=>this.marks[i]===turn).length===2)return empty[0];}return -1;};
    let pick=this.levelIndex>=2?lineFor(2):-1;
    if(pick<0&&this.levelIndex>=1)pick=lineFor(1);
    if(pick<0&&this.levelIndex>=2&&!this.marks[4])pick=4;
    const free=this.marks.map((mark,i)=>mark?-1:i).filter(i=>i>=0);
    if(pick<0)pick=free[(this.state.turns*5+this.levelIndex)%free.length];
    this.place(pick,2);
  }
  step() { if (this.builder) { super.step(0);return; }
    if(this.inputKeys.pressed('RIGHT'))this.selected=(this.selected+1)%9;
    if(this.inputKeys.pressed('LEFT'))this.selected=(this.selected+8)%9;
    if(this.inputKeys.pressed('DOWN'))this.selected=(this.selected+3)%9;
    if(this.inputKeys.pressed('UP'))this.selected=(this.selected+6)%9;
    this.player.setPosition(this.cells[this.selected].x,this.cells[this.selected].y);
    if(this.replyAt&&this.elapsed>=this.replyAt){this.replyAt=0;if(!this.state.ended)this.machineMove();}
  }
}
start(Board);
