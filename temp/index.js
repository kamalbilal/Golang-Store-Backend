const fs = require("fs")


let str = fs.readFileSync("./MT6781_Android_scatter.xml", "utf-8")
const result = [];
const regex = /<partition_name>(.*?)<\/partition_name>\s*<file_name>(.*?)<\/file_name>/g;
let match;
while ((match = regex.exec(str))) {
  if (match[2] !== 'NONE' && !result.some(obj => obj.partition_name === match[1])) {
    result.push({
      partition_name: match[1],
      file_name: match[2],
    });
  }
}
result.map((el) => {
    if (el.partition_name == "userdata") {
        return
    }
    console.log(`sudo ./fastboot flash ${el.partition_name} '/media/kali/Local Disk/All Phones Files/Tecno Camon 18P/CH7n-H812DE-R-OP-220527V1418/${el.file_name}'`);
})