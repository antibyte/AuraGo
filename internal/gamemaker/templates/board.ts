import { GameScene, start } from './common';
class Board extends GameScene {
  selected=0; cells:any[]=[]; marks:number[]=[];
  setup() {
    this.selected=0;this.marks=Array(9).fill(0);this.cells=[];
    for(let i=0;i<9;i++){
      const cell=this.add.rectangle(360+(i%3)*100,180+Math.floor(i/3)*100,90,90,0x334155).setInteractive();
      cell.on('pointerdown',()=>{this.selected=i;this.action();});this.cells.push(cell);
    }
    this.player=this.add.rectangle(360,180,96,96).setStrokeStyle(3,0xfacc15);
  }
  action() {
    if(this.marks[this.selected])return;
    const turn=this.state.turns%2+1;this.marks[this.selected]=turn;
    this.cells[this.selected].setFillStyle(turn===1?0x5eead4:0xfb7185);
    this.state.actions++;this.state.hits++;this.state.turns++;this.state.score++;
    const won=[[0,1,2],[3,4,5],[6,7,8],[0,3,6],[1,4,7],[2,5,8],[0,4,8],[2,4,6]].some(line=>line.every(i=>this.marks[i]===turn));
    if(won||this.state.turns===9)this.end();
  }
  step() {
    if(this.inputKeys.pressed('RIGHT'))this.selected=(this.selected+1)%9;
    if(this.inputKeys.pressed('LEFT'))this.selected=(this.selected+8)%9;
    if(this.inputKeys.pressed('DOWN'))this.selected=(this.selected+3)%9;
    if(this.inputKeys.pressed('UP'))this.selected=(this.selected+6)%9;
    this.player.setPosition(this.cells[this.selected].x,this.cells[this.selected].y);
  }
}
start(Board);
