## intint64_db

아비트라지 시스템을 위해
중단 허용·고정 크기·단일 actor 모델을 전제로 한
특수 목적 상태 저장 엔진을 설계·구현

### 사용법
```
make dbms
make client # dbms 와 다른 쉘에서 실행
```

### 용어사전

- id
  - 주소를 가리키는 값(혹은 주소 그 자체)
- value
  - 값을 조회한 결과, 커맨드의 타겟 값
- command
  - 영속성 데이터를 변경하는 클라이언트 명령
- query
  - 영속성 데이터를 조회하는 클라이언트 명령
- last_id
  - 0.0.0.value 를 통해 저장된 가장 마지막 id 를 가리키는 예약변수

### 특징

- int64 만으로 구성된 database
- id, value 의 형태로 저장됨
- udp 만으로 통신됨
- 일관성을 만족하지 않음(오직 어플리케이션 책임)
- 스토리지(영속성 관리)는 파일시스템을 활용
- 파일의 크기는 한번 정해지면 변하지 않음
- lock 을 사용하지 않음
- 단일 수정 쓰레드만 존재한다(액터모델)
- 시간 주기적으로 저장된다
  - 메타파일을 수정하여 관리된다(dbms 재시작 후 적용됨)
- 삭제 불가능함
  - 삭제라는 개념은 의도적으로 없어야한다, 기본적으로 모든 값은 0으로 저장되어 있다
  - 삭제를 의도한다면 0을 저장한다
  - null 의 의미를 상정해야한다면 0을 null 로 상정하고 0을 저장하지 못하도록 하라
  - 해당 시스템에 0과 null 모두가 의미가 있다면,
    - 이 db와 어울리지 않는다, 사용하지 마라

### 기본 환경
- port
  - 7770

### command & query rule

- 각 비트수는 64, 64, 64, 64 수이며, udp 포맷에서 전달될때는 256bit 가 한번에 전달되어야한다
- id 나 value의 범위가 넘어가지 않는지 선행 검사된다

### command

- . 으로 구분하여 명령 실행
- 커맨드의 종류는 버전별로 증가만 함
- 숫자 + . + 숫자 의 형태로 존재함
- id 는 주소를 가리키는 int64, value 는 값을 가리키는 int64 를 넣는다
- last_id 는 meta_ 가 붙은 파일에 저장된다

#### command format(client)

- 0.0.0.value : auto save
  - value 를 last_id+1 위치에 저장하고 last_id 를 1증가 시킨후 종료
- 0.1.id.value : target replace
  - id 에 값을 value로 저장하고 종료
- 0.2.id.value : 
  - id 에 값이 last_id 보다 크거나 같다면 수정하지 않고 종료, 아니라면 last_id 는 그대로 두고, 해당 id 에 value 를 저장하고 종료
- 0.3.id.value :
  - id 에 값이 last_id 보다 크거나 같다면 수정하고 last_id 를 해당 id 로 수정하고 해당 id 에 value 를 저장하고 종료, 아니라면 종료
- 0.4.id.value : 
  - id 에 값이 last_id 보다 크거나 같다면 수정하고 last_id 를 해당 id 로 수정하고 해당 id 에 value 를 저장하고 종료, 아니라면 last_id 만 해당 id 로 변경
- 0.5.n.value :
  - 현재 시간을 기준n으로 퀀타이즈된(초/분/시간 등) 인덱스로 값을 저장, last_id 는 수정하지 않고 종료
- 0.6.n.0 :
  - 시간 퀀타이즈 기준n을 초로 저장하고 종료
- 0.6.n.1 :
  - 시간 퀀타이즈 기준n을 분으로 저장하고 종료
- 0.6.n.2 :
  - 시간 퀀타이즈 기준n을 시간으로 저장하고 종료
- 0.6.n.3~62 :
  - 시간 퀀타이즈 기준n을 각 시간 0~59분으로 저장하고 종료


#### query format(client)

- 1.0.0.id : read one
  - id 의 value 를 출력
- 1.9.type.0 : read last call timestamp
  - 가장 최근 접수된 type 번호 명령의 실행 시간 출력
- 1.9.type.1 : read last call id
  - 가장 최근 접수된 type 번호 명령의 대상 id(혹은 id1) 출력
- 6.0.id1.id2 : range query
  - id1 부터 id2 까지(포함) value 를 순서대로 출력. 응답은 id당 1패킷(1.0.0.value)으로 여러 개 전송됨


### 나중에 확장 가능한 방향들

#### mutation format(client)

- 2.0.id1.id2 : 
  - id1 의 value 와 id2 의 value 중 큰 값은 id1에 저장, 작은 값은 id2에 저장하고 (큰값,작은값,큰값id,작은값id) 를 출력
- 

#### after-service format(client)

- 3.0.id.value :
  - id 에 값을 value 초 뒤에 0으로 변경
- 3.1.id.value :
  - id 에 값을 value 분 뒤에 0으로 변경

#### lazy-action format(client)

- 4.0.id1.id2 :
  - id1 에 값이 수정될때 id2 값도 동일하게 변경

#### trigger-action format(client)

- 5.0.id1.id2 :
  - id1 에서 4.0 으로 시작되는 명령이 실행될때 id2에도 전파

#### 